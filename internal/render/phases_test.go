package render

import (
	"strings"
	"testing"
)

var testHighlight = RGB{R: 0x00, G: 0x87, B: 0xff}

func plainPhases(phases []string, current, width int) PhasesOptions {
	return PhasesOptions{
		Phases:        phases,
		CurrentIndex:  current,
		PreviousIndex: current,
		Fade:          1,
		Width:         width,
		Profile:       ProfileNone,
		Highlight:     testHighlight,
	}
}

func TestRenderPhasesEmptyPlanRendersEmpty(t *testing.T) {
	got, target := RenderPhases(plainPhases(nil, 0, 40))
	if got != "" {
		t.Fatalf("RenderPhases() = %q, want empty string for empty plan", got)
	}
	if target != 0 {
		t.Fatalf("RenderPhases() target = %d, want 0", target)
	}
}

func TestRenderPhasesBracketsCurrentUnderProfileNone(t *testing.T) {
	got, _ := RenderPhases(plainPhases([]string{"build", "test", "ship"}, 1, 40))
	want := "build › [test] › ship"
	if strings.TrimRight(got, " ") != want {
		t.Fatalf("RenderPhases() = %q, want %q", got, want)
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("RenderPhases() = %q, want no escape codes under profile none", got)
	}
}

func TestRenderPhasesHighlightsCurrentWithBoldAndColour(t *testing.T) {
	opts := plainPhases([]string{"build", "test", "ship"}, 1, 40)
	opts.Profile = ProfileTrueColor
	got, _ := RenderPhases(opts)

	wantCurrent := Bold() + Foreground(ProfileTrueColor, testHighlight) + "test"
	if !strings.Contains(got, wantCurrent) {
		t.Fatalf("RenderPhases() = %q, want current phase styled as %q", got, wantCurrent)
	}
	if strings.Contains(StripANSI(got), "[") {
		t.Fatalf("RenderPhases() = %q, want no brackets when colour is available", got)
	}
	if StripANSI(strings.TrimRight(got, " ")) != "build › test › ship" {
		t.Fatalf("RenderPhases() stripped = %q", StripANSI(got))
	}
}

func TestRenderPhasesDimsPreviousAndPendingPhases(t *testing.T) {
	opts := plainPhases([]string{"build", "test", "ship"}, 1, 40)
	opts.Profile = ProfileTrueColor
	got, _ := RenderPhases(opts)

	if run := styleRunBefore(got, "build"); !strings.Contains(run, Dim()) {
		t.Fatalf("RenderPhases() = %q, want previous phase dimmed (run %q)", got, run)
	}
	if run := styleRunBefore(got, "ship"); !strings.Contains(run, Dim()) {
		t.Fatalf("RenderPhases() = %q, want pending phase dimmed (run %q)", got, run)
	}
}

// styleRunBefore returns the escape sequence that opened the style run
// containing text: everything between the last reset before text and text
// itself, with the visible characters removed.
func styleRunBefore(got, text string) string {
	idx := strings.Index(got, text)
	if idx < 0 {
		return ""
	}
	prefix := got[:idx]
	if reset := strings.LastIndex(prefix, Reset()); reset >= 0 {
		prefix = prefix[reset+len(Reset()):]
	}
	// Drop visible characters so only the opening escapes remain.
	var out strings.Builder
	for _, m := range ansiPattern.FindAllString(prefix, -1) {
		out.WriteString(m)
	}
	return out.String()
}

func TestRenderPhasesASCIISeparator(t *testing.T) {
	opts := plainPhases([]string{"build", "test", "ship"}, 0, 40)
	opts.ASCII = true
	got, _ := RenderPhases(opts)
	want := "[build] > test > ship"
	if strings.TrimRight(got, " ") != want {
		t.Fatalf("RenderPhases() = %q, want %q", got, want)
	}
}

func TestRenderPhasesPadsToBudget(t *testing.T) {
	got, _ := RenderPhases(plainPhases([]string{"a", "b"}, 0, 12))
	if got != "[a] › b     " {
		t.Fatalf("RenderPhases() = %q, want right-padded to 12 columns", got)
	}
}

func TestRenderPhasesClipsRightWithEllipsis(t *testing.T) {
	// strip: "[aaaa] › bbbb › cccc › dddd" (27 columns)
	opts := plainPhases([]string{"aaaa", "bbbb", "cccc", "dddd"}, 0, 10)
	opts.Offset = 0
	got, _ := RenderPhases(opts)
	if got != "[aaaa] › …" {
		t.Fatalf("RenderPhases() = %q, want right-clipped strip ending in ellipsis", got)
	}
}

func TestRenderPhasesClipsLeftWithEllipsis(t *testing.T) {
	// strip: "aaaa › bbbb › cccc › [dddd]" (27 columns); offset 17 shows the tail.
	opts := plainPhases([]string{"aaaa", "bbbb", "cccc", "dddd"}, 3, 10)
	opts.Offset = 17
	got, _ := RenderPhases(opts)
	if got != "… › [dddd]" {
		t.Fatalf("RenderPhases() = %q, want left-clipped strip starting with ellipsis", got)
	}
}

func TestRenderPhasesClipsBothSides(t *testing.T) {
	// strip: "aaaa › [bbbb] › cccc › dddd" (27 columns)
	opts := plainPhases([]string{"aaaa", "bbbb", "cccc", "dddd"}, 1, 12)
	opts.Offset = 5
	got, _ := RenderPhases(opts)
	if got != "… [bbbb] › …" {
		t.Fatalf("RenderPhases() = %q, want ellipsis on both sides", got)
	}
}

func TestRenderPhasesASCIIEllipsisTakesTwoCells(t *testing.T) {
	opts := plainPhases([]string{"aaaa", "bbbb", "cccc", "dddd"}, 0, 10)
	opts.ASCII = true
	got, _ := RenderPhases(opts)
	if got != "[aaaa] >.." {
		t.Fatalf("RenderPhases() = %q, want ascii ellipsis %q occupying the last two cells", got, "..")
	}
}

func TestRenderPhasesTargetOffsetIsZeroWhenStripFits(t *testing.T) {
	_, target := RenderPhases(plainPhases([]string{"build", "test", "ship"}, 2, 40))
	if target != 0 {
		t.Fatalf("target = %d, want 0 when the strip fits", target)
	}
}

func TestRenderPhasesTargetOffsetKeepsCurrentPhaseVisible(t *testing.T) {
	phases := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf"}
	for current := range phases {
		opts := plainPhases(phases, current, 20)
		_, target := RenderPhases(opts)

		// Render at the target and make sure the bracketed current phase
		// survives clipping intact.
		opts.Offset = target
		got, _ := RenderPhases(opts)
		want := "[" + phases[current] + "]"
		if !strings.Contains(got, want) {
			t.Fatalf("current=%d target=%d: RenderPhases() = %q, want to contain %q", current, target, got, want)
		}
		if len([]rune(got)) != 20 {
			t.Fatalf("current=%d: RenderPhases() = %q, want exactly 20 columns", current, got)
		}
	}
}

func TestRenderPhasesTargetOffsetShowsNeighbourWhenBudgetAllows(t *testing.T) {
	phases := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf"}
	opts := plainPhases(phases, 3, 30)
	_, target := RenderPhases(opts)
	opts.Offset = target
	got, _ := RenderPhases(opts)
	if !strings.Contains(got, "charlie") || !strings.Contains(got, "echo") {
		t.Fatalf("RenderPhases() = %q, want both neighbours of the current phase visible", got)
	}
}

func TestRenderPhasesTargetOffsetIsClampedToStrip(t *testing.T) {
	phases := []string{"alpha", "bravo", "charlie", "delta"}
	opts := plainPhases(phases, 3, 12)
	_, target := RenderPhases(opts)
	// strip: "alpha › bravo › charlie › [delta]" = 33 columns
	if target < 0 || target > 33-12 {
		t.Fatalf("target = %d, want within [0, %d]", target, 33-12)
	}
}

func TestRenderPhasesOffsetIsClampedWhenPastEnd(t *testing.T) {
	opts := plainPhases([]string{"aaaa", "bbbb"}, 1, 8)
	opts.Offset = 100
	got, _ := RenderPhases(opts)
	if len([]rune(got)) != 8 {
		t.Fatalf("RenderPhases() = %q, want 8 columns even with an out-of-range offset", got)
	}
	if !strings.Contains(got, "[bbbb]") {
		t.Fatalf("RenderPhases() = %q, want the tail of the strip", got)
	}
}

func TestRenderPhasesMeasuresWideRunes(t *testing.T) {
	// "構築" is 4 columns wide; "[構築] › b" is 10 columns.
	got, _ := RenderPhases(plainPhases([]string{"構築", "b"}, 0, 14))
	if got != "[構築] › b    " {
		t.Fatalf("RenderPhases() = %q, want wide runes counted as two columns each", got)
	}
}

func TestRenderPhasesCrossfadeBlendsColours(t *testing.T) {
	base := plainPhases([]string{"build", "test", "ship"}, 1, 40)
	base.Profile = ProfileTrueColor
	base.PreviousIndex = 0

	early := base
	early.Fade = 0.25
	got, _ := RenderPhases(early)
	// The previous phase carries a real colour escape, no dim and no bold.
	prev := styleRunBefore(got, "build")
	if strings.Contains(prev, Dim()) || strings.Contains(prev, Bold()) || !strings.Contains(prev, "\x1b[38;2;") {
		t.Fatalf("early fade: RenderPhases() = %q, want previous phase coloured without dim or bold (run %q)", got, prev)
	}
	// The current phase is coloured but not yet bold and not yet at the
	// full highlight colour.
	cur := styleRunBefore(got, "test")
	if strings.Contains(cur, Bold()) || cur == Foreground(ProfileTrueColor, testHighlight) {
		t.Fatalf("early fade: RenderPhases() = %q, want current phase mid-blend without bold (run %q)", got, cur)
	}
	if run := styleRunBefore(got, "ship"); !strings.Contains(run, Dim()) {
		t.Fatalf("early fade: RenderPhases() = %q, want pending phase dimmed (run %q)", got, run)
	}

	late := base
	late.Fade = 0.75
	got, _ = RenderPhases(late)
	if run := styleRunBefore(got, "test"); !strings.Contains(run, Bold()) {
		t.Fatalf("late fade: RenderPhases() = %q, want bold on the current phase once fade >= 0.5 (run %q)", got, run)
	}

	settled := base
	settled.Fade = 1
	got, _ = RenderPhases(settled)
	if run := styleRunBefore(got, "build"); !strings.Contains(run, Dim()) {
		t.Fatalf("settled: RenderPhases() = %q, want previous phase dim (run %q)", got, run)
	}
}

func TestRenderPhasesNoFadeUnderProfileNone(t *testing.T) {
	opts := plainPhases([]string{"build", "test", "ship"}, 1, 40)
	opts.PreviousIndex = 0
	opts.Fade = 0.1
	got, _ := RenderPhases(opts)
	if strings.TrimRight(got, " ") != "build › [test] › ship" {
		t.Fatalf("RenderPhases() = %q, want bracket form regardless of fade under profile none", got)
	}
}
