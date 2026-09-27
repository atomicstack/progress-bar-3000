package app

import (
	"bytes"
	"io"
	"strings"

	"golang.org/x/term"
)

// Erase-below (ED 0) in its two spellings, and the row-by-row replacement's
// building blocks. ESC 7 / ESC 8 are DECSC / DECRC: save and restore the
// cursor position and pen.
const (
	eraseBelow         = csi + "J"
	eraseBelowExplicit = csi + "0J"
	saveCursor         = "\x1b7"
	restoreCursor      = "\x1b8"
	cursorDownEraseRow = csi + "B" + eraseLine
	// fallbackEraseRows bounds the replacement when the terminal height is
	// unknown (output is not a TTY).
	fallbackEraseRows = 100
)

// ttyFile is the shape Bubble Tea type-asserts to recognise a terminal
// output; keeping it lets the renderer read the size and watch for resizes.
type ttyFile interface {
	io.ReadWriteCloser
	Fd() uintptr
}

// eraseRowsWriter rewrites every erase-below the renderer emits into an
// equivalent row-by-row erase. Bubble Tea v2 issues erase-below from the
// top row of the block on every full redraw (first frame, resize) and from
// its last row on shutdown. tmux treats erase-below issued from the home
// position as a full clear and scrolls the pane into its history, so in a
// dedicated pane each redraw would leave a stale copy of the bar in
// scrollback; see finalizeRenderedBlock for the same constraint.
//
// The rewrite relies on the renderer handing each frame to a single Write,
// so a sequence is never split across calls.
type eraseRowsWriter struct {
	io.Writer
	rows func() int
}

func (w eraseRowsWriter) Write(p []byte) (int, error) {
	if !bytes.Contains(p, []byte(eraseBelow)) && !bytes.Contains(p, []byte(eraseBelowExplicit)) {
		return w.Writer.Write(p)
	}
	replacement := eraseBelowRows(w.rows())
	out := bytes.ReplaceAll(p, []byte(eraseBelowExplicit), replacement)
	out = bytes.ReplaceAll(out, []byte(eraseBelow), replacement)
	if _, err := w.Writer.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// eraseBelowRows clears from the cursor to the end of its row, then each of
// the rows below it, and returns the cursor (and pen) to where it started.
// Cursor-down stops at the bottom margin, so surplus rows are harmless.
func eraseBelowRows(rows int) []byte {
	return []byte(saveCursor + eraseLineEnd + strings.Repeat(cursorDownEraseRow, max(0, rows-1)) + restoreCursor)
}

type eraseRowsFile struct {
	ttyFile
	eraseRowsWriter
}

func (f eraseRowsFile) Write(p []byte) (int, error) {
	return f.eraseRowsWriter.Write(p)
}

// withEraseRows wraps out in an eraseRowsWriter, preserving its terminal
// file methods when it has them.
func withEraseRows(out io.Writer) io.Writer {
	f, ok := out.(ttyFile)
	if !ok {
		return eraseRowsWriter{Writer: out, rows: func() int { return fallbackEraseRows }}
	}
	rows := func() int {
		if _, height, err := term.GetSize(int(f.Fd())); err == nil && height > 0 {
			return height
		}
		return fallbackEraseRows
	}
	return eraseRowsFile{ttyFile: f, eraseRowsWriter: eraseRowsWriter{Writer: f, rows: rows}}
}
