//go:build js && wasm

// Command wasm is the WebAssembly entry point. It exposes three functions on
// the JS global `goInspect`:
//
//	goInspect.checklist()            -> JSON string of the authored content
//	goInspect.computeReport(state)   -> JSON string of the scoring.Report
//	goInspect.renderReport(state,meta)-> HTML string of the printable report
//
// All inspection logic lives in Go; the frontend only renders and captures.
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/nulvox/inspection-checklist/internal/checklist"
	"github.com/nulvox/inspection-checklist/internal/report"
	"github.com/nulvox/inspection-checklist/internal/scoring"
)

var content = checklist.Load()

func main() {
	js.Global().Set("goInspect", js.ValueOf(map[string]any{
		"checklist":     js.FuncOf(checklistFn),
		"computeReport": js.FuncOf(computeReportFn),
		"renderReport":  js.FuncOf(renderReportFn),
	}))
	// Block forever so the exported functions stay alive.
	select {}
}

// checklistFn returns the embedded content JSON untouched.
func checklistFn(this js.Value, args []js.Value) any {
	return string(checklist.RawJSON())
}

// parseState decodes the state JSON passed from JS.
func parseState(arg js.Value) (scoring.State, error) {
	var st scoring.State
	if err := json.Unmarshal([]byte(arg.String()), &st); err != nil {
		return nil, err
	}
	return st, nil
}

// computeReportFn(stateJSON) -> reportJSON.
func computeReportFn(this js.Value, args []js.Value) any {
	st, err := parseState(args[0])
	if err != nil {
		return errJSON(err)
	}
	rep := scoring.Compute(content, st)
	out, err := json.Marshal(rep)
	if err != nil {
		return errJSON(err)
	}
	return string(out)
}

// renderReportFn(stateJSON, metaJSON) -> printable HTML.
func renderReportFn(this js.Value, args []js.Value) any {
	st, err := parseState(args[0])
	if err != nil {
		return errJSON(err)
	}
	var meta report.Meta
	if len(args) > 1 && args[1].Type() == js.TypeString {
		_ = json.Unmarshal([]byte(args[1].String()), &meta)
	}
	rep := scoring.Compute(content, st)
	html, err := report.Render(rep, meta)
	if err != nil {
		return errJSON(err)
	}
	return html
}

func errJSON(err error) string {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(b)
}
