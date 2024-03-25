package main

import (
  "os"
  "fmt"
  "log"
  "time"
  "strings"
  "net/http"

  "github.com/joho/godotenv"
)

var records [] ActivityRecord;

func main () {
  godotenv.Load()  // error silently

  err, _ := SttSync()
  if err != nil {
    log.Fatalln("sync error:", err)
  }

  stt_path := SttGetPath()
  csv_io_reader, err := os.Open(stt_path)
  if err != nil { return }
  defer csv_io_reader.Close()

  after_time   := time.Now().AddDate(0, 0, -7)
  records, err := SttCsvReadRange(csv_io_reader, &after_time, nil)

  if err != nil {
    log.Fatalln("STT parsing error:", err)
  }

  records = ActivityRecordsFilterCategories(records, "Productivity", "Development")

  fmt.Println("Number of records:", len(records))
  fmt.Println()

  http.HandleFunc("/", func (res http.ResponseWriter, req * http.Request) {
    serveIndex(res, req, records)
  })

  http.HandleFunc("/img.svg", func (res http.ResponseWriter, req * http.Request) {
    svg_string_builder := strings.Builder {}
    svg_string_builder.WriteString(
        ActivityRecordsPlotPieChart(records, nil),
      )

    res.Header().Set("Content-Type", "image/svg+xml")
    fmt.Fprintf(res, "%s", svg_string_builder.String())
  })

  fmt.Println("Server listening on port 8080...")
  http.ListenAndServe(":8080", nil)
}
