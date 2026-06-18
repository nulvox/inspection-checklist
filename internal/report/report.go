// Package report renders a scoring.Report into a printable HTML document. The
// user opens it and uses the browser's "Save as PDF" to keep a copy. No external
// libraries — pure Go + vanilla, fully offline.
//
// Photos are NOT embedded here (see adr/0002): a finding's photos render as
// <img data-photo-id> placeholders with no src. The frontend hydrates each src
// from IndexedDB (object URLs) before printing. A finding may carry several
// photos; the template ranges over them.
package report

import (
	"html/template"
	"strings"
	"time"

	"github.com/nulvox/inspection-checklist/internal/checklist"
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

// tallyColumns is the severity order shown across the per-phase tally table.
var tallyColumns = []checklist.Severity{
	checklist.DealBreaker, checklist.Concern, checklist.Moderate, checklist.Minor,
}

// severityLabel is the human-readable name for a severity in the report.
var severityLabel = map[checklist.Severity]string{
	checklist.DealBreaker: "Deal-breaker",
	checklist.Concern:     "Concern",
	checklist.Moderate:    "Moderate",
	checklist.Minor:       "Minor",
	checklist.OK:          "OK",
	checklist.NA:          "N/A",
}

// tallyRow is one phase's counts, aligned with tallyColumns, plus a row total.
type tallyRow struct {
	Phase  string
	Counts []int
	Total  int
}

// phaseFindings groups every flagged finding under the phase it belongs to,
// ordered by severity (deal-breakers first).
type phaseFindings struct {
	Name     string
	Findings []scoring.Finding
}

// view is the fully-prepared model handed to the template. Grouping and ordering
// happen here (in Go) so the template only iterates and prints.
type view struct {
	Meta         Meta
	Report       scoring.Report
	Disclaimer   string
	ColumnLabels []string
	Rows         []tallyRow
	PhaseGroups  []phaseFindings
}

// buildView reshapes a scoring.Report into the template model.
func buildView(rep scoring.Report, meta Meta) view {
	v := view{Meta: meta, Report: rep, Disclaimer: Disclaimer}

	for _, sev := range tallyColumns {
		v.ColumnLabels = append(v.ColumnLabels, severityLabel[sev])
	}

	// Findings ordered by severity, for slicing per phase.
	ordered := func() []scoring.Finding {
		var all []scoring.Finding
		all = append(all, rep.DealBreakers...)
		all = append(all, rep.Concerns...)
		all = append(all, rep.Moderate...)
		all = append(all, rep.Minor...)
		return all
	}()

	for _, pt := range rep.Phases {
		row := tallyRow{Phase: pt.Name}
		for _, sev := range tallyColumns {
			n := pt.Counts[sev]
			row.Counts = append(row.Counts, n)
			row.Total += n
		}
		v.Rows = append(v.Rows, row)

		var group []scoring.Finding
		for _, f := range ordered {
			if f.Phase == pt.Phase {
				group = append(group, f)
			}
		}
		if len(group) > 0 {
			v.PhaseGroups = append(v.PhaseGroups, phaseFindings{Name: pt.Name, Findings: group})
		}
	}
	return v
}

var reportTmpl = template.Must(template.New("report").
	Funcs(template.FuncMap{
		"joinTriggers": func(s []string) string { return strings.Join(s, "; ") },
		"sevLabel":     func(s checklist.Severity) string { return severityLabel[s] },
	}).
	Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<title>Inspection Report — {{.Meta.Property}}</title>
<style>
  :root{--db:#c0392b;--cn:#e67e22;--md:#b8860b;--mn:#7f8c8d;--rec:#2980b9}
  body{font:14px/1.55 system-ui,-apple-system,sans-serif;max-width:50rem;margin:2rem auto;padding:0 1rem;color:#1a1a1a}
  h1{margin-bottom:.1rem} h2{margin-top:2rem;border-bottom:2px solid #eee;padding-bottom:.25rem}
  .sub{color:#666;margin-top:0}
  table{border-collapse:collapse;width:100%;margin:.5rem 0}
  th,td{border:1px solid #ddd;padding:.4rem .6rem;text-align:left}
  th{background:#f7f7f7} td.num,th.num{text-align:center}
  .finding{margin:.6rem 0;padding-left:.75rem;border-left:4px solid #ccc}
  .finding.dealbreaker{border-left-color:var(--db)}
  .finding.concern{border-left-color:var(--cn)}
  .finding.moderate{border-left-color:var(--md)}
  .finding.minor{border-left-color:var(--mn)}
  .rec{margin:.6rem 0;padding-left:.75rem;border-left:4px solid var(--rec)}
  .badge{display:inline-block;font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.03em;padding:.05rem .4rem;border-radius:.25rem;color:#fff;vertical-align:middle}
  .badge.dealbreaker{background:var(--db)} .badge.concern{background:var(--cn)}
  .badge.moderate{background:var(--md)} .badge.minor{background:var(--mn)}
  .note{color:#555;font-style:italic;margin:.15rem 0 0}
  .photos{display:flex;flex-wrap:wrap;gap:.4rem;margin:.35rem 0}
  .photo{max-width:12rem;max-height:9rem;object-fit:cover;border:1px solid #ddd;border-radius:.25rem;background:#f3f3f3}
  @media print{.photo{max-width:9rem;max-height:7rem;page-break-inside:avoid}}
  .triggers{color:#555;font-size:13px;margin:.15rem 0 0}
  .none{color:#2e7d32}
  .disclaimer{margin-top:2.5rem;padding-top:1rem;border-top:1px solid #ccc;font-size:12px;color:#777}
  a{color:var(--rec)}
  .savehint{background:#eef4ff;border:1px solid #cdddff;color:#1a3a6b;padding:.5rem .75rem;border-radius:.4rem;font-size:13px;margin:.75rem 0}
  @media print{body{margin:0;max-width:none}.noprint{display:none}}
</style></head><body>

<h1>Home Inspection Report</h1>
<p class="sub">{{if .Meta.Property}}{{.Meta.Property}} — {{end}}{{.Meta.Date}}</p>

<p class="savehint noprint">📄 This report is a live page, not a saved file. To keep a copy (and to bundle any photos into it), use your browser's <strong>Print → Save as PDF</strong>.</p>

<h2>Summary by phase</h2>
<table>
  <thead><tr><th>Phase</th>{{range .ColumnLabels}}<th class="num">{{.}}</th>{{end}}<th class="num">Total</th></tr></thead>
  <tbody>
  {{range .Rows}}<tr><td>{{.Phase}}</td>{{range .Counts}}<td class="num">{{.}}</td>{{end}}<td class="num"><strong>{{.Total}}</strong></td></tr>{{end}}
  </tbody>
</table>

{{if .Report.DealBreakers}}
<h2 style="color:var(--db)">⛔ Deal-breakers ({{len .Report.DealBreakers}})</h2>
{{range .Report.DealBreakers}}<div class="finding dealbreaker"><strong>{{.Title}}</strong>{{if .Link}} — <a href="{{.Link.URL}}">{{.Link.Label}}</a>{{end}}{{if .Note}}<p class="note">{{.Note}}</p>{{end}}</div>{{end}}
{{else}}
<h2 class="none">✓ No deal-breakers flagged</h2>
{{end}}

{{if .Report.RollUps}}
<h2 style="color:var(--cn)">▲ Concern clusters ({{len .Report.RollUps}})</h2>
<p class="sub">No single item below is a deal-breaker, but several moderate findings piled up in one area — worth a closer look.</p>
{{range .Report.RollUps}}<div class="finding concern"><strong>{{.CategoryName}}</strong><p class="triggers">Moderate findings: {{joinTriggers .Contributing}}</p></div>{{end}}
{{end}}

{{if .Report.Recommendations}}
<h2 style="color:var(--rec)">🔍 Inspections to commission ({{len .Report.Recommendations}})</h2>
{{range .Report.Recommendations}}<div class="rec"><strong>{{.Item.Title}}</strong>{{if .Item.Detail}}<p class="note">{{.Item.Detail}}</p>{{end}}<p class="triggers">Triggered by: {{joinTriggers .TriggeredBy}}</p></div>{{end}}
{{end}}

{{if .PhaseGroups}}
<h2>Findings by phase</h2>
{{range .PhaseGroups}}
<h3>{{.Name}}</h3>
{{range .Findings}}<div class="finding {{.Severity}}"><span class="badge {{.Severity}}">{{sevLabel .Severity}}</span> <strong>{{.Title}}</strong>{{if .Link}} — <a href="{{.Link.URL}}">{{.Link.Label}}</a>{{end}}{{if .Note}}<p class="note">{{.Note}}</p>{{end}}{{if .PhotoIDs}}<div class="photos">{{range .PhotoIDs}}<img class="photo" data-photo-id="{{.}}" alt="finding photo">{{end}}</div>{{end}}</div>{{end}}
{{end}}
{{end}}

<p class="disclaimer">{{.Disclaimer}}</p>
</body></html>`))

// Render produces the printable HTML document.
func Render(rep scoring.Report, meta Meta) (string, error) {
	if meta.Date == "" {
		meta.Date = time.Now().Format("2006-01-02")
	}
	var b strings.Builder
	if err := reportTmpl.Execute(&b, buildView(rep, meta)); err != nil {
		return "", err
	}
	return b.String(), nil
}
