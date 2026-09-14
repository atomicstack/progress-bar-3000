package input

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"progress-bar-3000/internal/config"
)

func socketTestPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "pb3-ack-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "s.sock")
}

func TestSendRequiresValidAcknowledgement(t *testing.T) {
	for _, tc := range []struct {
		name, reply string
		ok          bool
	}{
		{"valid", `{"protocol":"progress-bar-3000/send-v1","ok":true,"count":1}`, true},
		{"negative", `{"protocol":"progress-bar-3000/send-v1","ok":false,"error":"bad event"}`, false},
		{"missing", "", false},
		{"wrong count", `{"protocol":"progress-bar-3000/send-v1","ok":true,"count":2}`, false},
		{"wrong protocol", `{"protocol":"other","ok":true,"count":1}`, false},
		{"garbage", "ok", false},
		{"contradictory", `{"protocol":"progress-bar-3000/send-v1","ok":true,"count":1,"error":"bad"}`, false},
		{"trailing json", `{"protocol":"progress-bar-3000/send-v1","ok":true,"count":1}{}`, false},
		{"stalled", "stall", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := socketTestPath(t)
			listener, err := net.Listen("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				_, _ = bufio.NewReader(conn).ReadString('\n')
				if tc.reply == "stall" {
					_, _ = io.Copy(io.Discard, conn)
				} else if tc.reply != "" {
					_, _ = io.WriteString(conn, tc.reply+"\n")
				}
			}()
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			err = Send(ctx, path, []string{"@tick"})
			if (err == nil) != tc.ok {
				t.Fatalf("send error=%v, want success=%v", err, tc.ok)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("fake server did not finish")
			}
		})
	}
}

func TestSendRejectsTrailingJSONEvents(t *testing.T) {
	err := Send(t.Context(), "/tmp/pb3-not-running.sock", []string{`{"type":"value","value":1}{"type":"value","value":2}`})
	if err == nil || !strings.Contains(err.Error(), "one json") {
		t.Fatalf("expected local rejection of multiple json objects, got %v", err)
	}
}

func TestOversizedSocketClientDoesNotStopRenderer(t *testing.T) {
	path := socketTestPath(t)
	source, err := NewUnixSocketSource(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- source.Run(ctx, func(string) error { return nil }) }()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	_, _ = io.WriteString(conn, strings.Repeat("x", MaxRequestBytes+2)+"\n")
	_, _ = bufio.NewReader(conn).ReadString('\n')
	_ = conn.Close()
	if err := Send(t.Context(), path, []string{"@value 1"}); err != nil {
		t.Fatalf("oversized client stopped renderer: %v", err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("source did not stop")
	}
}

func TestMalformedFramedClientDoesNotStopRenderer(t *testing.T) {
	path := socketTestPath(t)
	source, err := NewUnixSocketSource(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- source.Run(ctx, func(line string) error { _, err := ParseLine(config.InputModeAuto, line); return err })
	}()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	_, _ = io.WriteString(conn, `{"protocol":"progress-bar-3000/send-v1","events":["@tick"]`+"\n")
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	_ = conn.Close()
	if err != nil || !strings.Contains(string(line), `"ok":false`) {
		t.Fatalf("malformed batch did not receive rejection: %s %v", line, err)
	}
	if err := Send(t.Context(), path, []string{"@value 1"}); err != nil {
		t.Fatalf("malformed client stopped renderer: %v", err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("source did not stop")
	}
}

func TestSocketPathLengthExplainsPlatformLimit(t *testing.T) {
	limit := len(syscall.RawSockaddrUnix{}.Path) - 1
	for _, path := range []string{"/tmp/" + strings.Repeat("x", limit), "/tmp/" + strings.Repeat("é", limit/2+1)} {
		_, err := NewUnixSocketSource(path)
		if err == nil || !strings.Contains(err.Error(), "bytes") || !strings.Contains(err.Error(), "platform limit") || !strings.Contains(err.Error(), "shorter") {
			t.Fatalf("missing actionable path-length error for %d bytes: %v", len(path), err)
		}
	}
}

func TestSocketAcknowledgesVersionedBatchAfterEmission(t *testing.T) {
	path := socketTestPath(t)
	source, err := NewUnixSocketSource(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	emitted := make(chan string, 2)
	done := make(chan error, 1)
	go func() { done <- source.Run(ctx, func(line string) error { emitted <- line; return nil }) }()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	_, err = conn.Write([]byte(`{"protocol":"progress-bar-3000/send-v1","events":["@value 1","@phase-name test"]}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.(*net.UnixConn).CloseWrite()
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		t.Fatalf("renderer did not acknowledge batch: %v", err)
	}
	var ack struct {
		OK       bool   `json:"ok"`
		Count    int    `json:"count"`
		Protocol string `json:"protocol"`
	}
	if err := json.Unmarshal(line, &ack); err != nil {
		t.Fatal(err)
	}
	if !ack.OK || ack.Count != 2 || ack.Protocol != "progress-bar-3000/send-v1" {
		t.Fatalf("invalid ack: %s", line)
	}
	for _, want := range []string{"@value 1", "@phase-name test"} {
		select {
		case got := <-emitted:
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		default:
			t.Fatal("ack preceded event emission")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("source did not stop")
	}
}
