package checklist

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed checklist.json
var rawJSON []byte

// Load parses the embedded checklist content. It panics on malformed content,
// which can only happen if checklist.json is broken at build time.
func Load() *Checklist {
	var c Checklist
	if err := json.Unmarshal(rawJSON, &c); err != nil {
		panic(fmt.Sprintf("checklist: embedded checklist.json is invalid: %v", err))
	}
	return &c
}

// RawJSON returns the embedded content as-is, for handing straight to the
// frontend without a re-marshal.
func RawJSON() []byte { return rawJSON }
