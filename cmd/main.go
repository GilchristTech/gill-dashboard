package main

import (
  "os"
  "fmt"
  "log"
  "time"
  "strings"
  "net/http"

  "github.com/joho/godotenv"
  
  "gill-dashboard/pkg/stt_records"
  "gill-dashboard/web"
)

var records [] stt_records.ActivityRecord;

func main () {
  godotenv.Load()  // error silently

  err, _ := stt_records.SttSync()
  if err != nil {
    log.Fatalln("sync error:", err)
  }

  stt_path := stt_records.SttGetPath()
  csv_io_reader, err := os.Open(stt_path)
  if err != nil { return }
  defer csv_io_reader.Close()

  // Start by getting a year of records (year_records), determine when the
  // end-date of the downloaded amount is, and filter the final week of records
  // from there.

  today_start  := time.Now().Truncate(24 * time.Hour)

  year_window_start := today_start.AddDate(-1, 0, 0)
  year_records, err := stt_records.SttCsvReadRange(csv_io_reader, &year_window_start, nil)
  fmt.Println("Number of records, Year:", len(year_records))

  if err != nil {
    log.Fatalln("STT parsing error:", err)
  }

  // Determine the final record date. That will be the last date of the
  // week-long time window for the last-week metric.
  //
  var final_date time.Time = time.Time {}
  for _, record := range year_records {
    record_date := record.DayStart()
    if record_date.After(final_date) {
      final_date = record_date
    }
  }

  week_start := final_date.AddDate(0, 0, -7)

  records := stt_records.ActivityRecordsFilterTimeRange(year_records, &week_start, nil)
  fmt.Println("Number of records, Final Week:", len(records))
  records  = stt_records.ActivityRecordsFilterCategories(
      records, "Productivity", "Development",
    )

  fmt.Println("Number of records, Week Productivity:", len(records))
  fmt.Println()

  http.HandleFunc("/", func (res http.ResponseWriter, req * http.Request) {
    web.ServeIndex(res, req, records)
  })

  http.HandleFunc("/img.svg", func (res http.ResponseWriter, req * http.Request) {
    svg_string_builder := strings.Builder {}
    svg_string_builder.WriteString(
        stt_records.ActivityRecordsPlotPieChart(records, nil),
      )

    res.Header().Set("Content-Type", "image/svg+xml")
    fmt.Fprintf(res, "%s", svg_string_builder.String())
  })

  http.ListenAndServe(":8080", nil)
}
