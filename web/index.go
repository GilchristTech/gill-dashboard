package web


import (
  "embed"
  "fmt"
  "time"
  "net/http"
  "strings"
  htmlTemplate "html/template"
  textTemplate "text/template"

  stt "gill-dashboard/pkg/stt_records"
)

//go:embed templates/*.html
var templates       embed.FS
var base_template * textTemplate.Template

func init () {
  fmt.Println("Loading template")
  base_template = textTemplate.Must(textTemplate.ParseFS(templates, "templates/base.html"))
}


type BaseTemplate struct {
  Title string;
  Head  string;
  Main  string;
}


func pageHtmlIndex () {
  fmt.Println(htmlTemplate.HTMLEscape)
  fmt.Println(textTemplate.Must)
}


func ServeIndex (res http.ResponseWriter, req * http.Request, records [] stt.ActivityRecord) {
  // Iterate through records, and get the total number o
  records_duration := time.Duration(0)
  final_date       := time.Time {}
  for _, record := range records {
    records_duration += record.Duration
    record_date := record.DayStart()
    if record_date.After(final_date) {
      final_date = record_date
    }
  }

  main_builder := strings.Builder {}
  main_builder.WriteString(`<h1>Productivity: last seven days</h1>`)
  main_builder.WriteString("<figure>\n")
  main_builder.WriteString(stt.ActivityRecordsPlotPieChart(records, &stt.ActivityRecordChartOptions {
    Width: "100%",
    Height: "100%",
  }))
  main_builder.WriteString(`<figcaption>`)
  main_builder.WriteString(``)
  fmt.Fprintf(&main_builder, "<p>Number of records: %d\n</p>", len(records))
  fmt.Fprintf(&main_builder, "<p>Total duration: %s\n</p>", records_duration)
  y, m, d := final_date.Date()
  fmt.Fprintf(&main_builder, "<p>Final date: %d-%d-%d\n</p>", y, m, d)
  main_builder.WriteString(`</figcaption>`)
  main_builder.WriteString("</figure>")

  template_data := BaseTemplate {
    Title: "Home",
    Main: main_builder.String(),

    Head: `
    <style>
      h1 {
        border-bottom: 1px solid #8888;
        margin: 1vh;
      }

      main {
        display: flex;
        flex-direction: column;
        &> figure { flex-grow: 1 }
      }

      figure {
        margin: 0;
        padding: 1em;
        display: flex;
        flex-direction: row wrap;
        justify-content: center;

        &> svg {
          flex-grow: 1;
          object-fit: contain;
          object-position: top;
          max-height: 80dvh;
          width: auto;
        }
      }

      img {
        max-width: 100%;
      }
    </style>`,
  }

  base_template.Execute(res, template_data)
  // http.ServeFile(res, req, "index.html")
}
