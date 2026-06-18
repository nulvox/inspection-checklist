package scoring

import (
	"testing"

	"github.com/nulvox/inspection-checklist/internal/checklist"
)

// fixture builds a small, stable checklist for exercising Compute. It is
// deliberately independent of the authored checklist.json so that logic tests
// don't break when content is edited. Shape:
//
//	pre-viewing        -> pv-flood
//	first-walk         -> fw-marble (escalatesTo insp-structural)
//	                      fw-slope  (escalatesTo insp-structural)
//	                      fw-damp   (escalatesTo insp-roof)
//	                      fw-orphan (escalatesTo insp-missing -> no such item)
//	inspections-required -> insp-structural, insp-roof
func fixture() *checklist.Checklist {
	return &checklist.Checklist{
		Version: "test",
		Phases: []checklist.Phase{
			{
				ID:   checklist.PhasePreViewing,
				Name: "Pre-Viewing Diligence",
				Categories: []checklist.Category{{
					ID:   "diligence",
					Name: "Diligence",
					Items: []checklist.Item{
						{ID: "pv-flood", Title: "FEMA flood zone",
							Link: &checklist.Link{Label: "FEMA MSC", URL: "https://msc.fema.gov"}},
					},
				}},
			},
			{
				ID:   checklist.PhaseFirstWalk,
				Name: "First-Walk DIY Checks",
				Categories: []checklist.Category{{
					ID:   "structure",
					Name: "Structure",
					Items: []checklist.Item{
						{ID: "fw-marble", Title: "Marble on the floors", EscalatesTo: "insp-structural"},
						{ID: "fw-slope", Title: "Visible floor slope", EscalatesTo: "insp-structural"},
						{ID: "fw-damp", Title: "Damp on IR scope", EscalatesTo: "insp-roof"},
						{ID: "fw-orphan", Title: "Dangling escalation", EscalatesTo: "insp-missing"},
					},
				}},
			},
			{
				ID:   checklist.PhaseInspections,
				Name: "Inspections Required",
				Categories: []checklist.Category{{
					ID:   "specialists",
					Name: "Specialists",
					Items: []checklist.Item{
						{ID: "insp-structural", Title: "Structural engineer"},
						{ID: "insp-roof", Title: "Roofer"},
					},
				}},
			},
		},
	}
}

// Tracer bullet: a DealBreaker finding surfaces in DealBreakers with its title,
// note, and link resolved from the authored content.
func TestCompute_DealBreakerSurfaces(t *testing.T) {
	c := fixture()
	state := State{
		"pv-flood": {Severity: checklist.DealBreaker, Note: "in a floodway"},
	}

	rep := Compute(c, state)

	if len(rep.DealBreakers) != 1 {
		t.Fatalf("want 1 deal-breaker, got %d", len(rep.DealBreakers))
	}
	f := rep.DealBreakers[0]
	if f.ItemID != "pv-flood" {
		t.Errorf("ItemID = %q, want pv-flood", f.ItemID)
	}
	if f.Title != "FEMA flood zone" {
		t.Errorf("Title = %q, want resolved from content", f.Title)
	}
	if f.Note != "in a floodway" {
		t.Errorf("Note = %q, want carried from state", f.Note)
	}
	if f.Phase != checklist.PhasePreViewing {
		t.Errorf("Phase = %q, want pre-viewing", f.Phase)
	}
	if f.Link == nil || f.Link.URL != "https://msc.fema.gov" {
		t.Errorf("Link not resolved from content: %+v", f.Link)
	}
}

// Unchecked items (default) and NA items (doesn't apply) must not appear as
// findings and must not be counted in any phase tally. Items absent from the
// state map are treated as Unchecked.
func TestCompute_UncheckedAndNAExcluded(t *testing.T) {
	c := fixture()
	state := State{
		"pv-flood":  {Severity: checklist.Unchecked},
		"fw-marble": {Severity: checklist.NA},
		// fw-slope, fw-damp, fw-orphan, insp-* all absent => Unchecked.
	}

	rep := Compute(c, state)

	if n := len(rep.DealBreakers) + len(rep.Concerns) + len(rep.Moderate) + len(rep.Minor); n != 0 {
		t.Errorf("want no findings, got %d", n)
	}
	if len(rep.Recommendations) != 0 {
		t.Errorf("want no recommendations, got %d", len(rep.Recommendations))
	}
	for _, p := range rep.Phases {
		if len(p.Counts) != 0 {
			t.Errorf("phase %s tally should be empty, got %v", p.Phase, p.Counts)
		}
	}
}

// Each severity must land in its own bucket: Concern -> Concerns, Moderate ->
// Moderate, Minor -> Minor, DealBreaker -> DealBreakers. OK is a real status
// (looked, fine) but is not a finding.
func TestCompute_SeverityBucketing(t *testing.T) {
	c := fixture()
	state := State{
		"pv-flood":  {Severity: checklist.OK},
		"fw-marble": {Severity: checklist.Minor},
		"fw-slope":  {Severity: checklist.Moderate},
		"fw-damp":   {Severity: checklist.Concern},
		"fw-orphan": {Severity: checklist.DealBreaker},
	}

	rep := Compute(c, state)

	if len(rep.Minor) != 1 || rep.Minor[0].ItemID != "fw-marble" {
		t.Errorf("Minor bucket wrong: %+v", rep.Minor)
	}
	if len(rep.Moderate) != 1 || rep.Moderate[0].ItemID != "fw-slope" {
		t.Errorf("Moderate bucket wrong: %+v", rep.Moderate)
	}
	if len(rep.Concerns) != 1 || rep.Concerns[0].ItemID != "fw-damp" {
		t.Errorf("Concerns bucket wrong: %+v", rep.Concerns)
	}
	if len(rep.DealBreakers) != 1 || rep.DealBreakers[0].ItemID != "fw-orphan" {
		t.Errorf("DealBreakers bucket wrong: %+v", rep.DealBreakers)
	}
	// OK is counted in the tally but is never a finding.
	total := len(rep.Minor) + len(rep.Moderate) + len(rep.Concerns) + len(rep.DealBreakers)
	if total != 4 {
		t.Errorf("OK leaked into findings: total findings = %d, want 4", total)
	}
}

// Escalation fires only at Concern-or-worse. A Minor or Moderate finding on an
// item with an escalatesTo target must NOT surface the inspection.
func TestCompute_EscalationThreshold(t *testing.T) {
	tests := []struct {
		sev       checklist.Severity
		wantEscal bool
	}{
		{checklist.Minor, false},
		{checklist.Moderate, false},
		{checklist.Concern, true},
		{checklist.DealBreaker, true},
	}
	for _, tc := range tests {
		c := fixture()
		// fw-marble escalates to insp-structural.
		rep := Compute(c, State{"fw-marble": {Severity: tc.sev}})
		got := len(rep.Recommendations) == 1
		if got != tc.wantEscal {
			t.Errorf("severity %s: escalated=%v, want %v", tc.sev, got, tc.wantEscal)
		}
		if tc.wantEscal && rep.Recommendations[0].Item.ID != "insp-structural" {
			t.Errorf("severity %s: escalated to %q, want insp-structural",
				tc.sev, rep.Recommendations[0].Item.ID)
		}
	}
}

// Two findings that escalate to the SAME inspection must merge into one
// recommendation, with both triggering findings listed in TriggeredBy (no
// duplicate inspection entries).
func TestCompute_EscalationMergeDedup(t *testing.T) {
	c := fixture()
	// fw-marble and fw-slope both escalate to insp-structural.
	state := State{
		"fw-marble": {Severity: checklist.Concern},
		"fw-slope":  {Severity: checklist.DealBreaker},
	}

	rep := Compute(c, state)

	if len(rep.Recommendations) != 1 {
		t.Fatalf("want 1 merged recommendation, got %d: %+v", len(rep.Recommendations), rep.Recommendations)
	}
	rec := rep.Recommendations[0]
	if rec.Item.ID != "insp-structural" {
		t.Errorf("recommendation target = %q, want insp-structural", rec.Item.ID)
	}
	if len(rec.TriggeredBy) != 2 {
		t.Fatalf("want 2 triggers, got %d: %v", len(rec.TriggeredBy), rec.TriggeredBy)
	}
	want := map[string]bool{"Marble on the floors": false, "Visible floor slope": false}
	for _, tr := range rec.TriggeredBy {
		if _, ok := want[tr]; !ok {
			t.Errorf("unexpected trigger %q", tr)
		}
		want[tr] = true
	}
	for title, seen := range want {
		if !seen {
			t.Errorf("missing trigger %q in %v", title, rec.TriggeredBy)
		}
	}
}

// An escalatesTo pointing at an ID that doesn't exist must be ignored safely:
// no recommendation, no panic. (Guards against drifted IDs in content.)
func TestCompute_EscalationDanglingTargetIgnored(t *testing.T) {
	c := fixture()
	// fw-orphan escalates to "insp-missing", which has no item.
	rep := Compute(c, State{"fw-orphan": {Severity: checklist.DealBreaker}})

	if len(rep.Recommendations) != 0 {
		t.Errorf("dangling escalation should yield no recommendation, got %+v", rep.Recommendations)
	}
	// The finding itself still surfaces.
	if len(rep.DealBreakers) != 1 {
		t.Errorf("want the finding to still surface, got %d deal-breakers", len(rep.DealBreakers))
	}
}

// PhaseTally must count flagged items by severity, scoped to the phase the item
// belongs to. One Phase entry is emitted per phase, in content order.
func TestCompute_PhaseTallies(t *testing.T) {
	c := fixture()
	state := State{
		"pv-flood":  {Severity: checklist.DealBreaker},
		"fw-marble": {Severity: checklist.Concern},
		"fw-slope":  {Severity: checklist.Concern},
		"fw-damp":   {Severity: checklist.Minor},
	}

	rep := Compute(c, state)

	if len(rep.Phases) != 3 {
		t.Fatalf("want 3 phase tallies, got %d", len(rep.Phases))
	}
	byID := map[checklist.PhaseID]PhaseTally{}
	for _, p := range rep.Phases {
		byID[p.Phase] = p
	}

	if got := byID[checklist.PhasePreViewing].Counts[checklist.DealBreaker]; got != 1 {
		t.Errorf("pre-viewing dealbreaker count = %d, want 1", got)
	}
	if got := byID[checklist.PhaseFirstWalk].Counts[checklist.Concern]; got != 2 {
		t.Errorf("first-walk concern count = %d, want 2", got)
	}
	if got := byID[checklist.PhaseFirstWalk].Counts[checklist.Minor]; got != 1 {
		t.Errorf("first-walk minor count = %d, want 1", got)
	}
	// inspections-required had nothing flagged.
	if got := len(byID[checklist.PhaseInspections].Counts); got != 0 {
		t.Errorf("inspections tally should be empty, got %v", byID[checklist.PhaseInspections].Counts)
	}
	// Phases are emitted in content order.
	if rep.Phases[0].Phase != checklist.PhasePreViewing ||
		rep.Phases[1].Phase != checklist.PhaseFirstWalk ||
		rep.Phases[2].Phase != checklist.PhaseInspections {
		t.Errorf("phase order wrong: %v %v %v",
			rep.Phases[0].Phase, rep.Phases[1].Phase, rep.Phases[2].Phase)
	}
}
