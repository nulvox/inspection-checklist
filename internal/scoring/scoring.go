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
	// PhotoIDs reference user-captured images (Blobs in IndexedDB; see adr/0002).
	// A finding may have several — never bound to one. Empty when no photos.
	PhotoIDs []string `json:"photoIds,omitempty"`
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
	// PhotoIDs carries the finding's photo references through to the report,
	// where each becomes an <img data-photo-id> placeholder hydrated by JS.
	PhotoIDs []string `json:"photoIds,omitempty"`
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

// RollUp is a synthesized Concern raised when several Moderate findings pile up
// in one category. No single item is alarming, but the cluster warrants a closer
// look — so we surface it as a Concern without mutating the underlying findings.
type RollUp struct {
	Phase        checklist.PhaseID  `json:"phase"`
	Category     string             `json:"category"`     // category ID
	CategoryName string             `json:"categoryName"` // human-readable category name
	Severity     checklist.Severity `json:"severity"`     // always Concern
	Weight       int                `json:"weight"`       // summed weight of the contributing moderates
	Contributing []string           `json:"contributing"` // titles of the moderate findings that rolled up
}

// Report is the full decision-useful output.
type Report struct {
	DealBreakers    []Finding        `json:"dealBreakers"`
	Concerns        []Finding        `json:"concerns"` // severity Concern
	Moderate        []Finding        `json:"moderate"`
	Minor           []Finding        `json:"minor"`
	RollUps         []RollUp         `json:"rollUps"` // clusters of moderates surfaced as concerns
	Recommendations []Recommendation `json:"recommendations"`
	Phases          []PhaseTally     `json:"phases"`
}

// Roll-up tuning. A category surfaces a Concern when at least rollUpMinItems
// Moderate findings accumulate, AND their summed weight reaches rollUpThreshold.
// The item-count floor keeps a single heavy item from synthesizing a concern on
// its own ("several" means more than one); the weight sum lets authored content
// mark heavier items so a smaller pile still trips.
const (
	rollUpMinItems   = 2
	rollUpThreshold  = 3
	defaultItemWeight = 1
)

// itemWeight is the item's authored weight, defaulting to 1 when unset.
func itemWeight(it checklist.Item) int {
	if it.Weight <= 0 {
		return defaultItemWeight
	}
	return it.Weight
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
// Weighting: there is deliberately no composite score. Beyond bucketing findings
// by severity, the one derived signal is the roll-up — several Moderate findings
// clustered in a single category surface a synthesized Concern (see RollUp and
// the rollUp* constants). Per-item Weight lets authored content mark heavier
// items so a smaller cluster still trips the rule.
func Compute(c *checklist.Checklist, state State) Report {
	rep := Report{}
	// Recommendations are keyed by inspections-required item ID and merged.
	recByID := map[string]*Recommendation{}

	for _, phase := range c.Phases {
		tally := PhaseTally{Phase: phase.ID, Name: phase.Name, Counts: map[checklist.Severity]int{}}
		for _, cat := range phase.Categories {
			// Accumulate Moderate findings within this category for the roll-up rule.
			mod := RollUp{Phase: phase.ID, Category: cat.ID, CategoryName: cat.Name, Severity: checklist.Concern}
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
					PhotoIDs: st.PhotoIDs,
				}
				switch st.Severity {
				case checklist.DealBreaker:
					rep.DealBreakers = append(rep.DealBreakers, f)
				case checklist.Concern:
					rep.Concerns = append(rep.Concerns, f)
				case checklist.Moderate:
					rep.Moderate = append(rep.Moderate, f)
					mod.Weight += itemWeight(it)
					mod.Contributing = append(mod.Contributing, it.Title)
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
			if len(mod.Contributing) >= rollUpMinItems && mod.Weight >= rollUpThreshold {
				rep.RollUps = append(rep.RollUps, mod)
			}
		}
		rep.Phases = append(rep.Phases, tally)
	}

	for _, r := range recByID {
		rep.Recommendations = append(rep.Recommendations, *r)
	}
	return rep
}
