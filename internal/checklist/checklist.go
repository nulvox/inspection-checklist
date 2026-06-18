// Package checklist defines the data model for the home-inspection checklist
// and loads the authored content embedded in checklist.json.
//
// The JSON file is the single source of truth for *what* gets inspected. Runtime
// state (what the user found) lives separately — see package scoring.
package checklist

// Severity is the status a user assigns to an item during a walkthrough.
// It is NOT part of the authored content; it is captured at runtime.
type Severity string

const (
	Unchecked   Severity = "unchecked"   // not yet looked at (default)
	NA          Severity = "na"          // doesn't apply to this property
	OK          Severity = "ok"          // looked, fine
	Minor       Severity = "minor"       // cosmetic / cheap fix
	Moderate    Severity = "moderate"    // worth budgeting for
	Concern     Severity = "concern"     // get a pro to look before committing
	DealBreaker Severity = "dealbreaker" // walk-away unless price reflects it
)

// Link points at a procedure, standard, or how-to for an item.
type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Item is one thing to check.
type Item struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	// Tool names the kit needed (e.g. "marble", "outlet tester", "IR scope",
	// "fridge magnet", "nose"). Empty means no special tool.
	Tool string `json:"tool,omitempty"`
	// Unorthodox marks the DIY tricks that look odd to a normal buyer but are
	// legit. The UI badges these.
	Unorthodox bool `json:"unorthodox,omitempty"`
	// Link is an optional procedure / standard / how-to reference.
	Link *Link `json:"link,omitempty"`
	// EscalatesTo is the ID of an item in the inspections-required phase that
	// should be recommended when this item is flagged Concern or worse.
	// e.g. the marble/slope item escalates to "ir-structural".
	EscalatesTo string `json:"escalatesTo,omitempty"`
	// PhotoRef is reserved for v2 photo capture (see PLAN.md). Unused in v1.
	PhotoRef string `json:"photoRef,omitempty"`
}

// Category groups items within a phase.
type Category struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Items []Item `json:"items"`
}

// PhaseID identifies the three top-level inspection phases.
type PhaseID string

const (
	PhasePreViewing PhaseID = "pre-viewing"          // desk diligence
	PhaseFirstWalk  PhaseID = "first-walk"           // DIY, basic toolkit
	PhaseInspections PhaseID = "inspections-required" // pro / specialist
)

// Phase is one of the three stages of inspection.
type Phase struct {
	ID         PhaseID    `json:"id"`
	Name       string     `json:"name"`
	Blurb      string     `json:"blurb"`
	Categories []Category `json:"categories"`
}

// Checklist is the whole authored content tree.
type Checklist struct {
	Version string  `json:"version"`
	Phases  []Phase `json:"phases"`
}

// ItemByID returns the item with the given ID and the phase it belongs to.
func (c *Checklist) ItemByID(id string) (Item, PhaseID, bool) {
	for _, p := range c.Phases {
		for _, cat := range p.Categories {
			for _, it := range cat.Items {
				if it.ID == id {
					return it, p.ID, true
				}
			}
		}
	}
	return Item{}, "", false
}
