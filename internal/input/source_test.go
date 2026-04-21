package input

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNullSourceReturnsOnContextCancel(t *testing.T) {
	t.Parallel()

	source := NewNullSource()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- source.Run(ctx, func(string) error {
			t.Error("null source must not emit events")
			return nil
		})
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after ctx cancel")
	}

	if err := source.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestReaderSourceEmitsLines(t *testing.T) {
	t.Parallel()

	source := NewReaderSource(strings.NewReader("alpha\nbeta\n"))

	var mu sync.Mutex
	var got []string
	err := source.Run(context.Background(), func(line string) error {
		mu.Lock()
		got = append(got, line)
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := []string{"alpha", "beta"}
	if len(got) != len(want) {
		t.Fatalf("Run() emitted %d lines, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Run()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUnixSocketSourceRejectsExistingPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "input.sock")
	if err := os.WriteFile(path, []byte("occupied"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := NewUnixSocketSource(path)
	if err == nil {
		t.Fatal("NewUnixSocketSource() error = nil, want error")
	}
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("NewUnixSocketSource() error = %v, want os.ErrExist", err)
	}
}

func TestUnixSocketSourceWaitsForReconnect(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	path := "input.sock"

	source, err := NewUnixSocketSource(path)
	if err != nil {
		t.Fatalf("NewUnixSocketSource() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan error, 1)
	var mu sync.Mutex
	var got []string

	go func() {
		ready <- source.Run(ctx, func(line string) error {
			mu.Lock()
			got = append(got, line)
			mu.Unlock()
			return nil
		})
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("socket path %q was not created in time", path)
		}
		time.Sleep(10 * time.Millisecond)
	}

	writeLine := func(line string) {
		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("Dial() error = %v", err)
		}
		if _, err := conn.Write([]byte(line + "\n")); err != nil {
			_ = conn.Close()
			t.Fatalf("Write() error = %v", err)
		}
		if err := conn.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}

	writeLine("alpha")
	writeLine("beta")

	deadline = time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		count := len(got)
		lines := append([]string(nil), got...)
		mu.Unlock()
		if count == 2 {
			if lines[0] != "alpha" || lines[1] != "beta" {
				t.Fatalf("Run() emitted %#v, want [alpha beta]", lines)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Run() emitted %d lines, want 2: %#v", count, lines)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	if err := source.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case err := <-ready:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled or nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after Close()")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("socket path %q still exists after Close()", path)
	}
}

func TestUnixSocketSourceCancelsIdleClient(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}()

	path := "input.sock"

	source, err := NewUnixSocketSource(path)
	if err != nil {
		t.Fatalf("NewUnixSocketSource() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- source.Run(ctx, func(string) error { return nil })
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("socket path %q was not created in time", path)
		}
		time.Sleep(10 * time.Millisecond)
	}

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled or nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after context cancellation")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("socket path %q still exists after cancellation", path)
	}
}
