// Package report renders a scoring.Report into a self-contained, printable HTML
// document. The user opens it and uses the browser's "Save as PDF". No external
// libraries — keeps the build pure Go + vanilla and fully offline.
package report

import (
	"html/template"
	"strings"
	"time"

	"github.com/nulvox/inspection-checklist/internal/scoring"
)

// Meta is the header info for a report (property address, date, etc.).
type Meta struct {
	Property string
	Date     string
}

// Disclaimer is reprinted on every report (see decision: modal + footer + report).
const Disclaimer = `This checklist and report are for PERSONAL USE ONLY and are provided "AS IS", ` +
	`without warranties of any kind, express or implied. They are an informational aid, ` +
	`NOT professional advice, and NOT a substitute for a licensed home, structural, ` +
	`electrical, pest, or environmental inspection. The author assumes no liability for ` +
	`decisions made using this tool.`

// reportTmpl is intentionally minimal. TODO(next-agent): flesh out styling, add
// the per-phase tallies table, group findings by phase, and embed notes nicely.
var reportTmpl = template.Must(template.New("report").
	Funcs(template.FuncMap{
		"joinTriggers": func(s []string) string { return strings.Join(s, "; ") },
	}).
	Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>Inspection Report — {{.Meta.Property}}</title>
<style>
  body{font:14px/1.5 system-ui,sans-serif;max-width:48rem;margin:2rem auto;padding:0 1rem;color:#1a1a1a}
  h1{margin-bottom:0} .sub{color:#666}
  .dealbreaker{border-left:4px solid #c0392b;padding-left:.75rem}
  .concern{border-left:4px solid #e67e22;padding-left:.75rem}
  .rec{border-left:4px solid #2980b9;padding-left:.75rem}
  .finding{margin:.5rem 0} .note{color:#555;font-style:italic}
  .disclaimer{margin-top:2rem;padding-top:1rem;border-top:1px solid #ccc;font-size:12px;color:#777}
  @media print{.noprint{display:none}}
</style></head><body>
<h1>Home Inspection Report</h1>
<p class="sub">{{.Meta.Property}} — {{.Meta.Date}}</p>

{{if .Report.DealBreakers}}
<h2 class="dealbreaker">⛔ Deal-breakers ({{len .Report.DealBreakers}})</h2>
{{range .Report.DealBreakers}}<div class="finding"><strong>{{.Title}}</strong>{{if .Note}}<div class="note">{{.Note}}</div>{{end}}</div>{{end}}
{{else}}<h2>No deal-breakers flagged</h2>{{end}}

{{if .Report.Concerns}}<h2 class="concern">⚠ Concerns ({{len .Report.Concerns}})</h2>
{{range .Report.Concerns}}<div class="finding"><strong>{{.Title}}</strong>{{if .Note}}<div class="note">{{.Note}}</div>{{end}}</div>{{end}}{{end}}

{{if .Report.Recommendations}}<h2 class="rec">🔍 Inspections to commission ({{len .Report.Recommendations}})</h2>
{{range .Report.Recommendations}}<div class="finding"><strong>{{.Item.Title}}</strong> — triggered by: {{joinTriggers .TriggeredBy}}</div>{{end}}{{end}}

<p class="disclaimer">{{.Disclaimer}}</p>
</body></html>`))

// Render produces the printable HTML document.
func Render(rep scoring.Report, meta Meta) (string, error) {
	if meta.Date == "" {
		meta.Date = time.Now().Format("2006-01-02")
	}
	var b strings.Builder
	data := struct {
		Meta       Meta
		Report     scoring.Report
		Disclaimer string
	}{meta, rep, Disclaimer}
	if err := reportTmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
