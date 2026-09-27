package app

import (
	"bytes"
	"os"
	"testing"
)

func TestEraseRowsWriterRewritesEraseBelow(t *testing.T) {
	var out bytes.Buffer
	w := eraseRowsWriter{Writer: &out, rows: func() int { return 3 }}
	in := "\x1b[H\x1b[J" + "bar" + "\x1b[0J"

	n, err := w.Write([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if n != len(in) {
		t.Fatalf("Write() = %d, want the caller's length %d", n, len(in))
	}
	// Both spellings become: save, erase the rest of the row, step down and
	// erase each row below, restore. No ED 0 reaches the terminal, so tmux
	// never sees an erase-below from the home position.
	rows := "\x1b7\x1b[K" + "\x1b[B\x1b[2K" + "\x1b[B\x1b[2K" + "\x1b8"
	want := "\x1b[H" + rows + "bar" + rows
	if got := out.String(); got != want {
		t.Fatalf("Write() wrote %q, want %q", got, want)
	}
}

func TestEraseRowsWriterPassesOtherSequencesThrough(t *testing.T) {
	var out bytes.Buffer
	w := eraseRowsWriter{Writer: &out, rows: func() int { t.Fatal("rows() called without an erase-below"); return 0 }}
	in := "\x1b[2J\x1b[1J\x1b[K\x1b[2K\x1b[38;5;99mbar\x1b[m"

	if _, err := w.Write([]byte(in)); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != in {
		t.Fatalf("Write() wrote %q, want it unchanged", got)
	}
}

func TestEraseBelowRowsSingleRowSkipsCursorDown(t *testing.T) {
	for _, rows := range []int{-1, 0, 1} {
		if got, want := string(eraseBelowRows(rows)), "\x1b7\x1b[K\x1b8"; got != want {
			t.Fatalf("eraseBelowRows(%d) = %q, want %q", rows, got, want)
		}
	}
}

func TestWithEraseRowsKeepsTerminalFile(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	// Bubble Tea only reads the size and watches for resizes when its output
	// still exposes the file descriptor.
	f, ok := withEraseRows(w).(ttyFile)
	if !ok {
		t.Fatal("withEraseRows(*os.File) lost the terminal file methods")
	}
	if f.Fd() != w.Fd() {
		t.Fatalf("Fd() = %d, want %d", f.Fd(), w.Fd())
	}
	// A pipe has no size, so the fallback row count applies.
	if _, err := f.Write([]byte("\x1b[J")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(buf[:n]), string(eraseBelowRows(fallbackEraseRows)); got != want {
		t.Fatalf("pipe received %q, want %q", got, want)
	}
}

func TestWithEraseRowsPlainWriter(t *testing.T) {
	var out bytes.Buffer
	w := withEraseRows(&out)
	if _, ok := w.(ttyFile); ok {
		t.Fatal("withEraseRows(*bytes.Buffer) claims to be a terminal file")
	}
	if _, err := w.Write([]byte("\x1b[J")); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), string(eraseBelowRows(fallbackEraseRows)); got != want {
		t.Fatalf("Write() wrote %q, want %q", got, want)
	}
}
