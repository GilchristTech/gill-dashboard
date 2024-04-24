package stt_records


import (
  "fmt"
  "os"
  "io"
  "net/http"
  "errors"
  "time"
  "encoding/csv"
  "strconv"
  "strings"
  re "regexp"
  "math"
  "math/rand"
)


var STT_CSV_COLUMNS [] string = [] string {
  "activity name",
  "time started",
  "time ended",
  "comment",
  "categories",
  "record tags",
  "duration",
  "duration minutes",
}


var SttInitialized bool          = false
var STT_DAY_OFFSET time.Duration = 0
var STT_TIMEZONE * time.Location = nil

type ActivityRecord struct {
  /*
    Column data, as it exists in a STT export CSV file, unmarshalled
  */
  Activity_name    string;
  Time_started     time.Time;
  Time_ended       time.Time;
  Comment          string;
  Categories       [] string;
  Record_tags      string;
  Duration         time.Duration;
  Duration_minutes uint;
}


func SttInit () (err error) {
  day_offset_str, found := os.LookupEnv("STT_DAY_OFFSET")
  if found {
    STT_DAY_OFFSET, err = time.ParseDuration(day_offset_str)
    if err != nil { return err }
  }

  timezone_str, found := os.LookupEnv("STT_TIMEZONE")
  if found {
    STT_TIMEZONE, err = time.LoadLocation(timezone_str)
    if err != nil { return err }
  }

  SttInitialized = true
  return nil
}


func DayStart (datetime time.Time) (time.Time) {
  y, m, d := datetime.Add(-STT_DAY_OFFSET).Date()
  return time.Date(y, m, d, 0, 0, 0, 0, datetime.Location())
}


func (record *ActivityRecord) DayStart () time.Time {
  return DayStart(record.time_started)
}


type ActivityRecordChartOptions struct {
  Width  string;
  Height string;
}


func downloadFile (output_path, url string) (bytes_written int64, err error) {
  bytes_written = 0
  output_file, err := os.Create(output_path)
  defer output_file.Close()
  if err != nil { return }

  response, err := http.Get(url)
  defer response.Body.Close()
  if err != nil { return }

  if response.StatusCode != http.StatusOK {
    return 0, fmt.Errorf("bad status: %s", response.Status)
  }

  bytes_written, err = io.Copy(output_file, response.Body)
  return bytes_written, err
}


func minutesFormatDuration (minutes uint) string {
  return fmt.Sprintf("%dh%dm", minutes/60, minutes % 60)
}


func SttGetPath () string {
  stt_path, ok := os.LookupEnv("STT_PATH")
  if ! ok {
    return "stt_records.csv"
  }
  return stt_path
}


func SttSync () (err error, was_downloaded bool) {
  /*
    Download Simple Time Tracker records CSV, if either the file does not exist
    or if the local copy is at least an hour old.
  */

  if SttInitialized == false {
    err := SttInit()
    if err != nil { return err, false }
  }

  stt_url, ok := os.LookupEnv("STT_URL")

  if ! ok {
    return fmt.Errorf("Environment variable STT_URL not set"), false
  }

  stt_path := SttGetPath()

  was_downloaded  = false
  do_download    := false

  // Look at the file, if it exists, and whether to download the STT CSV file.

  if stat, err := os.Stat("stt_records.csv"); err == nil {
    // The file exists; if it's old enough, set the do_download variable flag
    now := time.Now()
    var stt_age_hours float64 = now.Sub(stat.ModTime()).Hours()
    do_download = ( stt_age_hours >= 1 )
  } else if errors.Is(err, os.ErrNotExist) {
    do_download = true
  } else {
    return err, false
  }

  // Do the download, if the flag is set

  if do_download {
    fmt.Println("Downloading STT records...")

    records_num_bytes, err := downloadFile(stt_path, stt_url)
    if err != nil {
      return fmt.Errorf("Could not download STT records:", err), false
    }

    was_downloaded = true
    fmt.Printf("Downloaded STT records (%d)\n", records_num_bytes)
  }

  return nil, was_downloaded
}


func SttCsvReadRange (
  io_reader    io.Reader,
  after_date  *time.Time,
  before_date *time.Time,
) (
  records [] ActivityRecord,
  err     error,
) {
  csv_reader := csv.NewReader(io_reader)

  header_row, err := csv_reader.Read()
  if err != nil { return }

  column_indices := make(map [string] int, 7)

  for column_i, column_name := range header_row {
    column_indices[column_name] = column_i
  }

  // Validate columns

  STT_CSV_DURATION_RGX := re.MustCompile(`(\d+):(\d{1,2}):(\d{1,2})`)

  for _, column_name := range STT_CSV_COLUMNS {
    _, column_exists := column_indices[column_name]

    if ! column_exists {
      return nil, fmt.Errorf("CSV missing column: \"%s\"", column_name)
    }
  }

  //
  // Parse through the CSV, adding elements within the after/before date range
  // to the records array.
  //

  for {
    row, err := csv_reader.Read()

    // Exit on errors, but just break the loop when done reading the file
    if err != nil {
      if errors.Is(err, io.EOF) {
        break
      }
      return records, err
    }

    // Parse the dates first, then do the filter; and do these before creating
    // the ActivityRecord struct for the records array to prevent extraneuous
    // parsing when data is filtered.

    var time_started,     time_ended      time.Time
    var time_started_err, time_ended_err  error

    if STT_TIMEZONE == nil {
      time_started, time_started_err = time.Parse(time.DateTime, row[column_indices["time started"]])
      time_ended,   time_ended_err   = time.Parse(time.DateTime, row[column_indices["time ended"]])
    } else {
      time_started, time_started_err = time.ParseInLocation(
          time.DateTime, row[column_indices["time started"]], STT_TIMEZONE,
        )
      time_ended, time_ended_err = time.ParseInLocation(
          time.DateTime, row[column_indices["time ended"]], STT_TIMEZONE,
        )
    }

    if time_started_err != nil { continue }
    if time_ended_err   != nil { continue }

    // Filter for records whose day is within the date range
    day_start := DayStart(time_started)
    if after_date  != nil && day_start.Before(*after_date) { continue }
    if before_date != nil && day_start.After(*before_date) { continue }

    // Create the ActivityRecord struct

    var activity_name string = row[column_indices["activity name"]]
    var comment       string = row[column_indices["comments"]]
    var record_tags   string = row[column_indices["record_tags"]]

    duration_minutes_signed, err := strconv.Atoi(row[column_indices["duration minutes"]])
    if err != nil { continue }
    if duration_minutes_signed < 0 {
      continue
    }
    duration_minutes := uint(duration_minutes_signed)

    duration, err := time.ParseDuration(
      STT_CSV_DURATION_RGX.ReplaceAllString(
        row[column_indices["duration"]],
        `${1}h${2}m${3}s`,
      ),
    )
    if err != nil { fmt.Println(err) ; continue }

    var categories [] string = strings.Split(row[column_indices["categories"]], ", ")

    record := ActivityRecord {
      activity_name:    activity_name,
      comment:          comment,
      time_started:     time_started,
      time_ended:       time_ended,
      categories:       categories,
      record_tags:      record_tags,
      duration:         duration,
      duration_minutes: duration_minutes,
    }

    records = append(records, record)
  }

  return records, nil
}


func ActivityRecordsFilterCategories (records [] ActivityRecord, categories ...string) [] ActivityRecord {
  filtered := make([] ActivityRecord, len(records))
  count    := 0

  RECORD_SEARCH:
  for _, record := range records {
    for _, record_category := range record.categories {
      for _, filter_category := range categories {
        if record_category == filter_category {
          filtered[count] = record
          count++
          continue RECORD_SEARCH
        }
      }
    }
  }

  return filtered[:count]
}


func ActivityRecordsFilterTimeRange (records [] ActivityRecord, after_date, before_date * time.Time) [] ActivityRecord {
  filtered := make([] ActivityRecord, len(records))
  count    := 0

  for _, record := range records {
    date := record.DayStart()
    if after_date  != nil && date.Before(*after_date) { continue }
    if before_date != nil && date.After(*before_date) { continue }
    filtered[count] = record
    count++
  }

  return filtered[:count]
}


func ActivityRecordsPlotPieChart (records [] ActivityRecord, options * ActivityRecordChartOptions) string {
  //
  // Get sums of minutes for each activity name, and calculate their "pie
  // slice" ratio by dividing them by the sum of all activity minutes across
  // records.
  //

  if options == nil {
    options = & ActivityRecordChartOptions {
      Width: "400",
      Height: "220",
    }
  }

  activity_minutes := make(map [string] uint)
  var records_duration uint = 0

  for _, record := range records {
    name                     := record.activity_name
    record_duration          := record.duration_minutes
    activity_duration, found := activity_minutes[name]

    if found {
      activity_minutes[name] = activity_duration + record_duration
    } else {
      activity_minutes[name] = record_duration
    }

    records_duration += record_duration
  }

  //
  // Generate an SVG root and styles
  //

  var pie_svg  strings.Builder

  if options.width != "" || options.height != "" {
    fmt.Fprintf(
      &pie_svg,
      `<svg width="%s" height="%s" viewBox="-2 -1.1 4 2.2" xmlns="http://www.w3.org/2000/svg">` + "\n" +
      "  <style>\n" +
      "    path { transition: all 0.25s; stroke-width: 0.01; stroke: #8880; }\n " +
      "    path:hover { transform: scale(1.075); filter: brightness(1.1); stroke-width: 0.01; stroke: #8888; }\n" +
      "  </style>\n",
      options.width,
      options.height,
    )
  } else {
    fmt.Fprintf(
      &pie_svg,
      `<svg viewBox="-2 -1.1 4 2.2" xmlns="http://www.w3.org/2000/svg">` + "\n" +
      "  <style>\n" +
      "    path { transition: all 1s; }\n " +
      "    path:hover { transform: scale(1.075); filter: brightness(1.1); stroke-width: 0.01; stroke: #888; }\n" +
      "  </style>\n",
    )
  }

  //
  // Generate activity pie slice geometry
  //

  type ChartSlice struct {
    name       string;
    minutes      uint;
    ratio     float64;
    angle     float64;
    center_t  float64;

    start_t, start_x, start_y float64;
    end_t,   end_x,   end_y   float64;

    fill       string;
  }

  pie := make([] ChartSlice, 0)
  var pie_head float64 = 0  // keep track of the angle as we create slices

  for name, minutes := range activity_minutes {
    slice := ChartSlice {
      name:    name,
      minutes: minutes,
      ratio:   float64(minutes) / float64(records_duration),
      start_t: pie_head,
    }

    slice.angle     = 2 * math.Pi * slice.ratio
    slice.end_t     = pie_head + slice.angle

    slice.center_t  = slice.start_t + slice.angle/2

    pie_head        = slice.end_t

    slice.start_x   = math.Cos(slice.start_t)
    slice.start_y   = math.Sin(slice.start_t)

    slice.end_x     = math.Cos(slice.end_t)
    slice.end_y     = math.Sin(slice.end_t)

    rand.Seed(time.Now().UnixNano())
    r := 128 + rand.Intn(100)
    g := 128 + rand.Intn(100)
    b := 128 + rand.Intn(100)
    slice.fill = fmt.Sprintf(`#%02x%02x%02x`, r % 256, g % 256, b % 256)

    pie = append(pie, slice)
  }

  //
  // Generate SVG slice paths
  //

  for _, slice := range pie {
    var large_arc_flag string = "1"
    if slice.end_t - slice.start_t <= math.Pi {
      large_arc_flag = "0"
    }

    fmt.Fprintf(
      &pie_svg,
      "  <!-- %s: %dm, %2.1f%%, %3.1f° -->\n" +
      `  <path d="M 0 0 L %f %f A 1 1 0 %s 0 %f %f Z" fill="%s"/>` + "\n",
      slice.name,
      slice.minutes,
      slice.ratio * 100,
      slice.ratio * 360,

      slice.end_x,   slice.end_y,
      large_arc_flag,
      slice.start_x, slice.start_y,
      slice.fill,
    )
  }
  fmt.Fprintf(&pie_svg, "\n")

  //
  // Generate SVG pie slice text labels
  //

  for _, slice := range pie {
    text_x := math.Cos(slice.center_t) * 1.5
    text_y := math.Min(math.Max(math.Sin(slice.center_t) * 1.35, -1.0), 0.9)

    fmt.Fprintf(
      &pie_svg,
      `  <text x="%f" y="%f" font-family="sans-serif" font-size="0.1" text-anchor="middle">%s (%2.1f%%)</text>` + "\n",
      text_x, text_y,
      slice.name, slice.ratio * 100,
    )

    fmt.Fprintf(
      &pie_svg,
      `  <text x="%f" y="%f" font-family="sans-serif" font-size="0.08" text-anchor="middle">%s</text>` + "\n",
      text_x, text_y + 0.1,
      minutesFormatDuration(slice.minutes),
    )

    fmt.Fprintf(&pie_svg, "\n")
  }


  pie_svg.WriteString(`</svg>`)
  return pie_svg.String()
}
