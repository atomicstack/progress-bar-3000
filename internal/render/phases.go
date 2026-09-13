package render

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// PhasesOptions describes one frame of the %{phases} strip. The caller owns
// every time-dependent input (the pulsed highlight colour, the crossfade
// fraction and the lerped scroll offset) so this renderer stays pure.
type PhasesOptions struct {
	Phases        []string
	Subphase      string
	CurrentIndex  int
	PreviousIndex int
	// Fade is the crossfade progress from PreviousIndex to CurrentIndex:
	// 0 at the moment of the transition, 1 (or more) once settled.
	Fade float64
	// Width is the column budget for the strip.
	Width int
	// Offset is the first strip column to display, already lerped by the
	// caller. It is clamped to the strip.
	Offset    int
	Profile   Profile
	Highlight RGB
	ASCII     bool
}

type cellClass int

const (
	cellDim cellClass = iota
	cellHighlight
	cellSeparator
)

// cell is one display column of the rasterised strip. A wide rune occupies
// a head cell (width 2, carrying the text) followed by a continuation cell
// (empty text) so column arithmetic stays one-cell-per-column.
type cell struct {
	text         string
	width        int
	continuation bool
	class        cellClass
	phase        int
}

// cellStyle is the resolved escape state for a run of cells.
type cellStyle struct {
	dim   bool
	bold  bool
	color RGB
	// colored reports whether color should be emitted at all.
	colored bool
}

// fadeGrey is the neutral colour phases blend through during a crossfade.
var fadeGrey = RGB{R: 128, G: 128, B: 128}

const fadeBoldThreshold = 0.5

// RenderPhases renders the phase plan as a single-line strip with the
// current phase highlighted, clipped to a sliding window of opts.Width
// columns starting at opts.Offset. It returns the text for this frame and
// the offset the caller should scroll toward so the current phase (and,
// budget permitting, its neighbours) is in view.
func RenderPhases(opts PhasesOptions) (text string, targetOffset int) {
	if len(opts.Phases) == 0 || opts.Width <= 0 {
		return "", 0
	}

	cells, spans := rasterisePhases(opts)
	stripWidth := len(cells)
	ellipsisWidth := 1
	if opts.ASCII {
		ellipsisWidth = 2
	}

	targetOffset = phasesTargetOffset(opts, spans, stripWidth, ellipsisWidth)

	maxOffset := stripWidth - opts.Width
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + opts.Width
	if end > stripWidth {
		end = stripWidth
	}
	visible := append([]cell(nil), cells[offset:end]...)

	leftClipped := offset > 0
	rightClipped := end < stripWidth
	ellipsis := ellipsisCells(opts.ASCII)
	if leftClipped {
		replaceCells(visible, 0, ellipsis)
	}
	if rightClipped {
		replaceCells(visible, len(visible)-len(ellipsis), ellipsis)
	}

	return styleCells(opts, visible, opts.Width), targetOffset
}

// span is the half-open column range one phase name occupies in the strip.
type span struct {
	start int
	end   int
}

func rasterisePhases(opts PhasesOptions) ([]cell, []span) {
	separator := " › "
	if opts.ASCII {
		separator = " > "
	}
	bracket := opts.Profile == ProfileNone

	var cells []cell
	spans := make([]span, len(opts.Phases))
	for i, name := range opts.Phases {
		if i > 0 {
			cells = appendText(cells, separator, cellSeparator, -1)
		}
		class := cellDim
		if i == opts.CurrentIndex {
			class = cellHighlight
		}
		start := len(cells)
		if bracket && class == cellHighlight {
			cells = appendText(cells, "[", class, i)
		}
		cells = appendText(cells, name, class, i)
		if bracket && class == cellHighlight {
			cells = appendText(cells, "]", class, i)
		}
		if class == cellHighlight && opts.Subphase != "" {
			cells = appendText(cells, " ["+opts.Subphase+"]", class, i)
		}
		spans[i] = span{start: start, end: len(cells)}
	}
	return cells, spans
}

func appendText(cells []cell, text string, class cellClass, phase int) []cell {
	for _, r := range text {
		w := runewidth.RuneWidth(r)
		switch {
		case w <= 0:
			// Combining marks attach to the previous cell.
			if len(cells) > 0 {
				cells[len(cells)-1].text += string(r)
			}
		case w == 1:
			cells = append(cells, cell{text: string(r), width: 1, class: class, phase: phase})
		default:
			cells = append(cells, cell{text: string(r), width: w, class: class, phase: phase})
			for range w - 1 {
				cells = append(cells, cell{continuation: true, width: 0, class: class, phase: phase})
			}
		}
	}
	return cells
}

func ellipsisCells(ascii bool) []cell {
	if ascii {
		return []cell{
			{text: ".", width: 1, class: cellSeparator, phase: -1},
			{text: ".", width: 1, class: cellSeparator, phase: -1},
		}
	}
	return []cell{{text: "…", width: 1, class: cellSeparator, phase: -1}}
}

// replaceCells overwrites cells starting at index with replacement, then
// repairs any wide rune the overwrite cut in half so no column goes missing.
func replaceCells(cells []cell, index int, replacement []cell) {
	if index < 0 {
		index = 0
	}
	for i, r := range replacement {
		if index+i < len(cells) {
			cells[index+i] = r
		}
	}
	after := index + len(replacement)
	if after < len(cells) && cells[after].continuation {
		// The head of this wide rune was overwritten; show a blank column.
		cells[after] = cell{text: " ", width: 1, class: cells[after].class, phase: cells[after].phase}
	}
	if index > 0 && cells[index-1].width > 1 {
		// The tail of this wide rune was overwritten; the head can no longer
		// be drawn in a single column.
		cells[index-1] = cell{text: " ", width: 1, class: cells[index-1].class, phase: cells[index-1].phase}
	}
}

// phasesTargetOffset picks the window start that keeps the current phase
// fully visible, showing its neighbours (or at least their separators) when
// the budget allows, clamped to the strip.
func phasesTargetOffset(opts PhasesOptions, spans []span, stripWidth, ellipsisWidth int) int {
	if stripWidth <= opts.Width {
		return 0
	}
	current := opts.CurrentIndex
	if current < 0 || current >= len(spans) {
		return 0
	}

	// Clipping on either side spends ellipsisWidth columns, so only that
	// much of the budget is reliably available for context.
	budget := opts.Width - 2*ellipsisWidth
	cur := spans[current]
	lo, hi := cur.start, cur.end

	separatorWidth := 3
	candidates := [][2]int{}
	if current > 0 && current+1 < len(spans) {
		candidates = append(candidates, [2]int{spans[current-1].start, spans[current+1].end})
	}
	if current > 0 {
		candidates = append(candidates, [2]int{spans[current-1].start, hi})
	}
	if current+1 < len(spans) {
		candidates = append(candidates, [2]int{lo, spans[current+1].end})
	}
	sepLo, sepHi := lo, hi
	if current > 0 {
		sepLo = lo - separatorWidth
	}
	if current+1 < len(spans) {
		sepHi = hi + separatorWidth
	}
	candidates = append(candidates, [2]int{sepLo, sepHi})
	for _, c := range candidates {
		if c[1]-c[0] <= budget {
			lo, hi = c[0], c[1]
			break
		}
	}

	// Centre the chosen range in the window, then clamp to the strip.
	offset := lo - (opts.Width-(hi-lo))/2
	maxOffset := stripWidth - opts.Width
	offset = clampInt(offset, 0, maxOffset)

	// The ellipsis eats the edge cells; nudge so the current phase itself
	// is never underneath it.
	if offset > 0 && cur.start < offset+ellipsisWidth {
		offset = clampInt(cur.start-ellipsisWidth, 0, maxOffset)
	}
	if offset+opts.Width < stripWidth && cur.end > offset+opts.Width-ellipsisWidth {
		offset = clampInt(cur.end+ellipsisWidth-opts.Width, 0, maxOffset)
	}
	return offset
}

func styleCells(opts PhasesOptions, cells []cell, width int) string {
	var out strings.Builder
	var current cellStyle
	styled := false
	columns := 0

	for _, c := range cells {
		if c.continuation {
			continue
		}
		style := resolveStyle(opts, c)
		if opts.Profile != ProfileNone && (!styled || style != current) {
			if styled {
				out.WriteString(Reset())
			}
			out.WriteString(styleEscape(opts.Profile, style))
			current = style
			styled = true
		}
		out.WriteString(c.text)
		columns += c.width
	}
	if styled {
		out.WriteString(Reset())
	}
	if columns < width {
		out.WriteString(strings.Repeat(" ", width-columns))
	}
	return out.String()
}

func resolveStyle(opts PhasesOptions, c cell) cellStyle {
	if opts.Profile == ProfileNone {
		return cellStyle{}
	}
	fading := opts.Fade < 1 && opts.PreviousIndex != opts.CurrentIndex
	f := clampUnit(opts.Fade)

	switch {
	case c.class == cellHighlight:
		if fading {
			return cellStyle{bold: f >= fadeBoldThreshold, colored: true, color: lerpRGB(fadeGrey, opts.Highlight, f)}
		}
		return cellStyle{bold: true, colored: true, color: opts.Highlight}
	case c.class == cellDim && fading && c.phase == opts.PreviousIndex:
		return cellStyle{colored: true, color: lerpRGB(opts.Highlight, fadeGrey, f)}
	default:
		return cellStyle{dim: true}
	}
}

func styleEscape(profile Profile, style cellStyle) string {
	var out strings.Builder
	if style.dim {
		out.WriteString(Dim())
	}
	if style.bold {
		out.WriteString(Bold())
	}
	if style.colored {
		out.WriteString(Foreground(profile, style.color))
	}
	return out.String()
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
