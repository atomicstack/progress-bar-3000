package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
	"progress-bar-3000/internal/progress"
)

func applySubphaseLine(t *testing.T, m Model, line string) Model {
	t.Helper()
	evt, err := input.ParseLine(config.InputModeAuto, line)
	if err != nil {
		t.Fatalf("parse %q: %v", line, err)
	}
	updated, _ := m.Update(eventMsg{Event: evt, Now: time.Unix(100, 0)})
	return updated.(Model)
}

func TestSubphaseLifecycle(t *testing.T) {
	m := NewModel(config.Config{Format: "%{phase}|%{subphase}", FPS: 60}, progress.State{})
	for _, step := range []struct{ line, want string }{
		{"@set-phases build,test,ship", "build|"},
		{"@set-subphases compile, link", "build [compile]|compile"},
		{"@subphase 1", "build [link]|link"},
		{"@subphase-name compile", "build [compile]|compile"},
		{"@subphase-name unknown", "build [compile]|compile"},
		{"@subphase -1", "build [compile]|compile"},
		{"@subphase 99", "build [compile]|compile"},
		{"@subphase 1", "build [link]|link"},
		{`{"type":"set_subphases","phase":"test","subphases":["unit","integration"]}`, "build [link]|link"},
		{`{"type":"subphase","phase":"test","name":"integration"}`, "build [link]|link"},
		{"@phase-name test", "test [integration]|integration"},
		{`{"type":"phase","name":"missing","subphase":"unit"}`, "test [integration]|integration"},
		{`{"type":"subphase","name":"integration","index":0}`, "test [integration]|integration"},
		{"@phase-name build", "build [link]|link"},
		{`{"type":"set_subphases","phase":"missing","subphases":["wrong"]}`, "build [link]|link"},
		{`{"type":"subphase","phase":"missing","index":0}`, "build [link]|link"},
		{"@set-subphases recompile,relink", "build [recompile]|recompile"},
		{"@subphase 1", "build [relink]|relink"},
		{"@reset", "build [recompile]|recompile"},
		{"@phase-name test", "test [unit]|unit"},
		{"@set-subphases", "test|"},
		{"@subphase 0", "test|"},
		{"@phase-name build", "build [recompile]|recompile"},
		{`{"type":"reset","phases":null}`, "build [recompile]|recompile"},
		{`{"type":"set_subphases","subphases":[]}`, "build|"},
		{`{"type":"set_subphases","subphases":["compile, link"]}`, "build [compile, link]|compile, link"},
		{"@set-phases build,test", "build|"},
		{"@phase-name test", "test|"},
		{`{"type":"reset","phases":[]}`, "|"},
		{"@set-subphases orphan", "|"},
	} {
		m = applySubphaseLine(t, m, step.line)
		if got := m.View(); got != step.want {
			t.Fatalf("after %s: view = %q, want %q", step.line, got, step.want)
		}
		if m.state.Value != 0 {
			t.Fatalf("subphase command changed progress: %v", m.state.Value)
		}
	}
}

func TestSubphaseCombinedProgressUpdates(t *testing.T) {
	for _, tc := range []struct {
		name, line, wantPhase string
		wantValue             float64
	}{
		{"absolute with parent override", `{"type":"value","value":42,"phase":"build","subphase":"link"}`, "build [link]", 42},
		{"absolute with automatic parent", `{"type":"value","value":2,"subphase":"integration"}`, "test [integration]", 2},
		{"tick with parent override", `{"type":"tick","amount":0.5,"phase":"test","subphase":"integration"}`, "test [integration]", 0.5},
		{"tick current parent", `{"type":"tick","subphase":"link"}`, "build [link]", 1},
		{"phase with child", `{"type":"phase","name":"test","subphase":"integration"}`, "test [integration]", 0},
		{"phase index with child", `{"type":"phase","index":1,"subphase":"integration"}`, "test [integration]", 0},
		{"progress only", `{"type":"value","value":42}`, "ship", 42},
		{"invalid child keeps selection", `{"type":"value","value":0.5,"subphase":"missing"}`, "build [compile]", 0.5},
		{"invalid parent does not select child elsewhere", `{"type":"value","value":0.5,"phase":"missing","subphase":"link"}`, "build [compile]", 0.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(config.Config{Format: "%{phase}", FPS: 60}, progress.State{})
			m = applySubphaseLine(t, m, `{"type":"reset","total":100,"phases":[{"name":"build","subphases":["compile","link"]},{"name":"test","subphases":["unit","integration"]},"ship"]}`)
			m = applySubphaseLine(t, m, tc.line)
			if got := m.View(); got != tc.wantPhase || m.state.Value != tc.wantValue || m.state.Total != 100 {
				t.Fatalf("view = %q, value = %v, total = %d; want %q, %v, 100", got, m.state.Value, m.state.Total, tc.wantPhase, tc.wantValue)
			}
			if m.state.DisplayValue != 0 {
				t.Fatal("combined update bypassed progress smoothing")
			}
		})
	}
}

func TestSubphaseBootstrapAndRendering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phases.json")
	if err := os.WriteFile(path, []byte(`["fetch",{"name":"build","subphases":["compile","link"]},"test","ship"]`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mode  config.ColorMode
		ascii bool
		want  string
	}{
		{config.ColorModeTrueColor, false, "fetch › build [compile] › test › ship"},
		{config.ColorModeTrueColor, true, "fetch > build [compile] > test > ship"},
		{config.ColorModeNone, false, "fetch › [build] [compile] › test › ship"},
		{config.ColorModeNone, true, "fetch > [build] [compile] > test > ship"},
	} {
		cfg := config.Config{PhaseFile: path, Phase: "build", Format: "%{phases}", ColorMode: tc.mode, ASCII: tc.ascii, Detail: "phase", FPS: 60}
		state, err := bootstrapState(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if state.CurrentPhase() != "build" || state.Total != 4 {
			t.Fatalf("bootstrap changed parent identity or total: %q, %d", state.CurrentPhase(), state.Total)
		}
		m := NewModel(cfg, state)
		rows := strings.Split(stripANSI(m.View()), "\n")
		if strings.TrimSpace(rows[0]) != tc.want || rows[1] != "phase: build [compile]" {
			t.Fatalf("view = %q, want %q and phase detail", rows, tc.want)
		}
	}
}

func TestSubphaseUpdatesPreserveCompletionLifecycle(t *testing.T) {
	var output bytes.Buffer
	m := NewModel(config.Config{OnComplete: "printf x", FPS: 60}, progress.State{})
	m.hooks.output = &output
	for _, line := range []string{
		"@set-phases build,test", "@set-subphases compile,link", "@subphase 1",
	} {
		m = applySubphaseLine(t, m, line)
	}
	if m.state.EffectiveTotal() != 2 || m.completionFired {
		t.Fatal("subphase selection changed total or fired completion")
	}
	m = applySubphaseLine(t, m, `{"type":"value","value":2,"phase":"build","subphase":"link"}`)
	if !m.completionFired {
		t.Fatal("combined completion did not fire hook")
	}
	for _, line := range []string{"@set-subphases cleanup", "@subphase 0", "@value 2"} {
		m = applySubphaseLine(t, m, line)
	}
	if err := m.hooks.wait(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "x" {
		t.Fatalf("completion output = %q, want exactly one invocation", output.String())
	}
}
