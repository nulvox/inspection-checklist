package checklist

import (
	"net/url"
	"strings"
	"testing"
)

// These tests validate the authored checklist.json against the data model and
// the structural invariants the rest of the app relies on. They run against the
// real embedded content, so they fail the build when content drifts (a renamed
// ID, a dangling escalation, a malformed link).

// Load must parse the embedded content into the three known phases, in order.
func TestContent_LoadsThreePhases(t *testing.T) {
	c := Load()
	want := []PhaseID{PhasePreViewing, PhaseFirstWalk, PhaseInspections}
	if len(c.Phases) != len(want) {
		t.Fatalf("got %d phases, want %d", len(c.Phases), len(want))
	}
	for i, w := range want {
		if c.Phases[i].ID != w {
			t.Errorf("phase[%d] = %q, want %q", i, c.Phases[i].ID, w)
		}
		if strings.TrimSpace(c.Phases[i].Name) == "" {
			t.Errorf("phase %q has empty Name", c.Phases[i].ID)
		}
	}
	if strings.TrimSpace(c.Version) == "" {
		t.Error("checklist Version is empty")
	}
}

// Every item must have a non-empty ID and Title, and IDs must be unique across
// the whole tree (ItemByID and escalation both depend on uniqueness).
func TestContent_ItemIDsUniqueAndPopulated(t *testing.T) {
	c := Load()
	seen := map[string]bool{}
	forEachItem(c, func(it Item, _ PhaseID) {
		if strings.TrimSpace(it.ID) == "" {
			t.Errorf("item with empty ID (title %q)", it.Title)
			return
		}
		if strings.TrimSpace(it.Title) == "" {
			t.Errorf("item %q has empty Title", it.ID)
		}
		if seen[it.ID] {
			t.Errorf("duplicate item ID %q", it.ID)
		}
		seen[it.ID] = true
	})
}

// Every escalatesTo must resolve to an existing item, and that target must live
// in the inspections-required phase (escalation surfaces an inspection).
func TestContent_EscalationsResolveToInspections(t *testing.T) {
	c := Load()
	forEachItem(c, func(it Item, _ PhaseID) {
		if it.EscalatesTo == "" {
			return
		}
		target, phase, found := c.ItemByID(it.EscalatesTo)
		if !found {
			t.Errorf("item %q escalatesTo %q, which does not exist", it.ID, it.EscalatesTo)
			return
		}
		if phase != PhaseInspections {
			t.Errorf("item %q escalatesTo %q in phase %q, want %q",
				it.ID, target.ID, phase, PhaseInspections)
		}
	})
}

// Links, where present, must have a non-empty label and an absolute http(s) URL.
func TestContent_LinksWellFormed(t *testing.T) {
	c := Load()
	forEachItem(c, func(it Item, _ PhaseID) {
		if it.Link == nil {
			return
		}
		if strings.TrimSpace(it.Link.Label) == "" {
			t.Errorf("item %q has a link with no label", it.ID)
		}
		u, err := url.Parse(it.Link.URL)
		if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") {
			t.Errorf("item %q has a malformed link URL: %q", it.ID, it.Link.URL)
		}
	})
}

// forEachItem walks every item in the content tree.
func forEachItem(c *Checklist, fn func(Item, PhaseID)) {
	for _, p := range c.Phases {
		for _, cat := range p.Categories {
			for _, it := range cat.Items {
				fn(it, p.ID)
			}
		}
	}
}
