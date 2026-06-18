package report

import (
	"strings"
	"testing"

	"github.com/nulvox/inspection-checklist/internal/checklist"
	"github.com/nulvox/inspection-checklist/internal/scoring"
)

// sampleReport builds a representative scoring.Report covering every section the
// HTML report must render.
func sampleReport() scoring.Report {
	return scoring.Report{
		DealBreakers: []scoring.Finding{
			{ItemID: "fw-foundation", Title: "Active foundation movement",
				Phase: checklist.PhaseFirstWalk, Severity: checklist.DealBreaker,
				Note: "marble rolled fast toward the east wall"},
		},
		Concerns: []scoring.Finding{
			{ItemID: "pv-flood", Title: "FEMA flood zone AE",
				Phase: checklist.PhasePreViewing, Severity: checklist.Concern,
				Note: "in the 100-year floodplain"},
		},
		Moderate: []scoring.Finding{
			{ItemID: "fw-gfci", Title: "No GFCI in kitchen",
				Phase: checklist.PhaseFirstWalk, Severity: checklist.Moderate},
		},
		Minor: []scoring.Finding{
			{ItemID: "fw-paint", Title: "Fresh paint patch over ceiling",
				Phase: checklist.PhaseFirstWalk, Severity: checklist.Minor,
				Note: "could be hiding a stain"},
		},
		RollUps: []scoring.RollUp{
			{Phase: checklist.PhaseFirstWalk, Category: "fw-structure",
				CategoryName: "Structure & Floors", Severity: checklist.Concern, Weight: 4,
				Contributing: []string{"Foundation & brick cracks", "Doors out of square"}},
		},
		Recommendations: []scoring.Recommendation{
			{Item: checklist.Item{ID: "ir-structural", Title: "Structural engineer"},
				TriggeredBy: []string{"Active foundation movement", "Visible floor slope"}},
		},
		Phases: []scoring.PhaseTally{
			{Phase: checklist.PhasePreViewing, Name: "Pre-Viewing Diligence",
				Counts: map[checklist.Severity]int{checklist.Concern: 1}},
			{Phase: checklist.PhaseFirstWalk, Name: "First-Walk DIY Checks",
				Counts: map[checklist.Severity]int{
					checklist.DealBreaker: 1, checklist.Moderate: 1, checklist.Minor: 1}},
			{Phase: checklist.PhaseInspections, Name: "Inspections Required",
				Counts: map[checklist.Severity]int{}},
		},
	}
}

// Tracer: every report reprints the disclaimer and carries the property/date
// header (disclaimer is a hard requirement — modal + footer + report).
func TestRender_HeaderAndDisclaimer(t *testing.T) {
	html, err := Render(sampleReport(), Meta{Property: "123 Distressed Ln", Date: "2026-06-18"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(html, "123 Distressed Ln") {
		t.Error("property address missing from report")
	}
	if !strings.Contains(html, "2026-06-18") {
		t.Error("date missing from report")
	}
	if !strings.Contains(html, "NOT a substitute for a licensed") {
		t.Error("disclaimer missing from report")
	}
}

// The report must include a per-phase tally table: a table listing each phase
// name alongside severity columns.
func TestRender_PhaseTallyTable(t *testing.T) {
	html, err := Render(sampleReport(), Meta{Property: "x"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(html, "<table") {
		t.Error("tally table missing (<table not found)")
	}
	for _, phase := range []string{"Pre-Viewing Diligence", "First-Walk DIY Checks", "Inspections Required"} {
		if !strings.Contains(html, phase) {
			t.Errorf("phase %q missing from tally table", phase)
		}
	}
	for _, col := range []string{"Deal-breaker", "Concern", "Moderate", "Minor"} {
		if !strings.Contains(html, col) {
			t.Errorf("severity column %q missing from tally table", col)
		}
	}
}

// Every flagged finding — at every severity, including Moderate and Minor — must
// appear with its title and any note. The old template dropped Moderate/Minor.
func TestRender_AllFindingsAndNotesRendered(t *testing.T) {
	html, err := Render(sampleReport(), Meta{Property: "x"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	titles := []string{
		"Active foundation movement", // dealbreaker
		"FEMA flood zone AE",         // concern
		"No GFCI in kitchen",         // moderate
		"Fresh paint patch over ceiling", // minor
	}
	for _, title := range titles {
		if !strings.Contains(html, title) {
			t.Errorf("finding %q missing from report", title)
		}
	}
	notes := []string{
		"marble rolled fast toward the east wall",
		"in the 100-year floodplain",
		"could be hiding a stain",
	}
	for _, note := range notes {
		if !strings.Contains(html, note) {
			t.Errorf("note %q missing from report", note)
		}
	}
}

// Recommendations must render with the findings that escalated to them.
func TestRender_RecommendationsWithReasons(t *testing.T) {
	html, err := Render(sampleReport(), Meta{Property: "x"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(html, "Structural engineer") {
		t.Error("recommendation title missing")
	}
	if !strings.Contains(html, "Active foundation movement") ||
		!strings.Contains(html, "Visible floor slope") {
		t.Error("escalation reasons (triggered-by) missing from recommendation")
	}
}

// A roll-up (a cluster of moderate findings surfaced as a concern) must render
// with its category name and the findings that contributed to it.
func TestRender_RollUpsRendered(t *testing.T) {
	html, err := Render(sampleReport(), Meta{Property: "x"})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(html, "Structure &amp; Floors") {
		t.Error("roll-up category name missing from report")
	}
	if !strings.Contains(html, "Foundation &amp; brick cracks") ||
		!strings.Contains(html, "Doors out of square") {
		t.Error("roll-up contributing findings missing from report")
	}
}

// A clean report (nothing flagged) renders without panic and states there are no
// deal-breakers, while still printing the tally table and disclaimer.
func TestRender_EmptyReport(t *testing.T) {
	empty := scoring.Report{
		Phases: []scoring.PhaseTally{
			{Phase: checklist.PhasePreViewing, Name: "Pre-Viewing Diligence",
				Counts: map[checklist.Severity]int{}},
		},
	}
	html, err := Render(empty, Meta{})
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(html, "No deal-breakers flagged") {
		t.Error("empty report should state no deal-breakers flagged")
	}
	if !strings.Contains(html, "<table") {
		t.Error("empty report should still show the tally table")
	}
	if !strings.Contains(html, "NOT a substitute for a licensed") {
		t.Error("empty report still must carry the disclaimer")
	}
}
