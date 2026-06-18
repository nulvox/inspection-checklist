// Package scoring turns a user's per-item findings into a decision-useful
// report: deal-breakers first, then weighted concerns, then the inspections
// each finding escalates to.
//
// Design (settled): NO single composite score. A home is not a number — one
// cracked heat exchanger or active foundation movement is a walk-away no matter
// how nice the kitchen is. So we surface deal-breakers separately from a tally
// of concerns by severity.
package scoring

import "github.com/nulvox/inspection-checklist/internal/checklist"

// ItemState is what the user captured for one item.
type ItemState struct {
	Severity checklist.Severity `json:"severity"`
	Note     string             `json:"note"`
}

// State maps item ID -> what the user found. Items absent from the map are
// treated as Unchecked.
type State map[string]ItemState

// Finding is a flagged item, resolved against the content for titles/links.
type Finding struct {
	ItemID   string             `json:"itemId"`
	Title    string             `json:"title"`
	Phase    checklist.PhaseID  `json:"phase"`
	Severity checklist.Severity `json:"severity"`
	Note     string             `json:"note"`
	Link     *checklist.Link    `json:"link,omitempty"`
}

// Recommendation is an inspections-required item surfaced because a first-walk
// finding escalated to it.
type Recommendation struct {
	Item       checklist.Item `json:"item"`
	TriggeredBy []string       `json:"triggeredBy"` // titles of the findings that escalated here
}

// PhaseTally counts flagged items per severity within a phase.
type PhaseTally struct {
	Phase  checklist.PhaseID            `json:"phase"`
	Name   string                       `json:"name"`
	Counts map[checklist.Severity]int   `json:"counts"`
}

// Report is the full decision-useful output.
type Report struct {
	DealBreakers    []Finding        `json:"dealBreakers"`
	Concerns        []Finding        `json:"concerns"` // severity Concern
	Moderate        []Finding        `json:"moderate"`
	Minor           []Finding        `json:"minor"`
	Recommendations []Recommendation `json:"recommendations"`
	Phases          []PhaseTally     `json:"phases"`
}

// severityRank orders severities for "concern or worse" comparisons.
var severityRank = map[checklist.Severity]int{
	checklist.Unchecked:   0,
	checklist.NA:          0,
	checklist.OK:          1,
	checklist.Minor:       2,
	checklist.Moderate:    3,
	checklist.Concern:     4,
	checklist.DealBreaker: 5,
}

// atLeastConcern reports whether s is Concern or DealBreaker (the threshold
// that fires an escalation).
func atLeastConcern(s checklist.Severity) bool {
	return severityRank[s] >= severityRank[checklist.Concern]
}

// Compute builds the report from authored content + user state.
//
// TODO(next-agent): refine weighting once real walkthroughs exist — e.g. let
// content carry per-item weights, distinguish "concern" thresholds per category,
// and decide whether multiple moderate findings in one system should roll up to
// a concern. The structure below is intentionally simple and correct, not final.
func Compute(c *checklist.Checklist, state State) Report {
	rep := Report{}
	// Recommendations are keyed by inspections-required item ID and merged.
	recByID := map[string]*Recommendation{}

	for _, phase := range c.Phases {
		tally := PhaseTally{Phase: phase.ID, Name: phase.Name, Counts: map[checklist.Severity]int{}}
		for _, cat := range phase.Categories {
			for _, it := range cat.Items {
				st, ok := state[it.ID]
				if !ok || st.Severity == checklist.Unchecked || st.Severity == checklist.NA {
					continue
				}
				tally.Counts[st.Severity]++

				f := Finding{
					ItemID:   it.ID,
					Title:    it.Title,
					Phase:    phase.ID,
					Severity: st.Severity,
					Note:     st.Note,
					Link:     it.Link,
				}
				switch st.Severity {
				case checklist.DealBreaker:
					rep.DealBreakers = append(rep.DealBreakers, f)
				case checklist.Concern:
					rep.Concerns = append(rep.Concerns, f)
				case checklist.Moderate:
					rep.Moderate = append(rep.Moderate, f)
				case checklist.Minor:
					rep.Minor = append(rep.Minor, f)
				}

				// Escalation: a concern-or-worse finding recommends its target.
				if it.EscalatesTo != "" && atLeastConcern(st.Severity) {
					target, _, found := c.ItemByID(it.EscalatesTo)
					if found {
						r, exists := recByID[target.ID]
						if !exists {
							r = &Recommendation{Item: target}
							recByID[target.ID] = r
						}
						r.TriggeredBy = append(r.TriggeredBy, it.Title)
					}
				}
			}
		}
		rep.Phases = append(rep.Phases, tally)
	}

	for _, r := range recByID {
		rep.Recommendations = append(rep.Recommendations, *r)
	}
	return rep
}
