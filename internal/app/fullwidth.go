package app

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	fmtx "progress-bar-3000/internal/format"
)

func renderTemplate(t fmtx.Template, r resolver) string {
	if !r.cfg.WidthFull {
		return t.Render(r)
	}
	columns := r.termWidth
	if columns <= 0 {
		columns = defaultTerminalWidth
	}
	lines := t.Lines()
	rows := make([]string, len(lines))
	for i, line := range lines {
		rows[i] = renderFullWidthLine(line, r, columns)
	}
	return strings.Join(rows, "\n")
}

func renderFullWidthLine(t fmtx.Template, r resolver, columns int) string {
	// Measure the surrounding text and explicit-width bars first, then share
	// the remaining columns equally between bars without a width prefix.
	measure := &fullWidthResolver{resolver: r, measure: true}
	text := t.Render(measure)
	remaining := max(0, columns-ansi.StringWidth(text))
	fit := &fullWidthResolver{resolver: r, remaining: remaining, bars: measure.bars}
	return ansi.Truncate(t.Render(fit), columns, "")
}

type fullWidthResolver struct {
	resolver
	measure   bool
	remaining int
	bars      int
}

func (r *fullWidthResolver) Resolve(name string, width int) string {
	if width > 0 || (name != "progress" && name != "bar-only") {
		return r.resolver.Resolve(name, width)
	}
	if r.measure {
		r.bars++
		return ""
	}
	columns := r.remaining / r.bars
	r.remaining -= columns
	r.bars--
	if columns == 0 {
		return ""
	}
	// Custom background glyphs may occupy two cells. Keep each allocated bar
	// within its display-column budget and preserve the text that follows it.
	bar := ansi.Truncate(r.resolver.Resolve(name, columns), columns, "")
	return bar + strings.Repeat(" ", max(0, columns-ansi.StringWidth(bar)))
}
