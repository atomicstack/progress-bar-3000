package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"progress-bar-3000/internal/config"
	fmtx "progress-bar-3000/internal/format"
)

func TestFullWidthFitsRenderedRow(t *testing.T) {
	for _, tc := range []struct {
		name, format string
		columns      int
		wantWidth    int
	}{
		{"bare", "%p", 80, 80},
		{"percent", "%p %{percent}", 80, 80},
		{"resize", "%p %{percent}", 120, 120},
		{"fallback", "%p %{percent}", 0, 80},
		{"unicode", "界é %p %{percent}", 20, 20},
		{"ansi", "\x1b[31mred\x1b[0m %p %{percent}", 20, 20},
		{"repeated", "%p | %{bar-only} %{percent}", 20, 20},
		{"mixed widths", "%7{progress} %p %{percent}", 20, 20},
		{"explicit width", "%7{bar-only} %{percent}", 80, 11},
		{"no room", "%p %{percent}", 4, 4},
		{"tiny", "%p %{percent}", 1, 1},
		{"text exceeds viewport", "long label %p %{percent}", 5, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := barWidthModel(tc.format, 0)
			m.cfg.WidthFull = true
			tpl, err := fmtx.Parse(tc.format)
			if err != nil {
				t.Fatal(err)
			}
			got := renderTemplate(tpl, resolver{cfg: m.cfg, state: m.state, termWidth: tc.columns})
			if w := ansi.StringWidth(got); w != tc.wantWidth {
				t.Fatalf("row width = %d, want %d: %q", w, tc.wantWidth, got)
			}
			if tc.columns >= 20 && strings.Contains(tc.format, "%{percent}") && !strings.HasSuffix(got, "50%") {
				t.Fatalf("percentage missing: %q", got)
			}
		})
	}
}

func TestFullWidthMeasuresColoredWideBackground(t *testing.T) {
	m := barWidthModel("%p %{percent}", 0)
	m.cfg.WidthFull = true
	m.cfg.ColorMode = config.ColorModeTrueColor
	m.cfg.BackgroundStyle = config.BackgroundStyleCustom
	m.cfg.BackgroundRune = '界'
	tpl, err := fmtx.Parse(m.cfg.Format)
	if err != nil {
		t.Fatal(err)
	}
	got := renderTemplate(tpl, resolver{cfg: m.cfg, state: m.state, termWidth: 20})
	if w := ansi.StringWidth(got); w != 20 || !strings.HasSuffix(got, "50%") {
		t.Fatalf("row must fit with percentage intact: width = %d, row = %q", w, got)
	}
}

func TestFullWidthBudgetsEachTemplateLine(t *testing.T) {
	m := barWidthModel("%p %{percent}\n%p %{phase}\n\n%7{progress} %%\n", 0)
	m.cfg.WidthFull = true
	m.state.Phases = []string{"build"}
	tpl, err := fmtx.Parse(m.cfg.Format)
	if err != nil {
		t.Fatal(err)
	}
	got := renderTemplate(tpl, resolver{cfg: m.cfg, state: m.state, termWidth: 30})
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("line count = %d, want 5: %q", len(lines), got)
	}
	for i, want := range []int{30, 30, 0, 9, 0} {
		if width := ansi.StringWidth(lines[i]); width != want {
			t.Errorf("line %d width = %d, want %d: %q", i, width, want, lines[i])
		}
	}
	if !strings.HasSuffix(lines[0], "50%") || !strings.HasSuffix(lines[1], "build") || !strings.HasSuffix(lines[3], " %") {
		t.Errorf("line content lost: %q", got)
	}
}
