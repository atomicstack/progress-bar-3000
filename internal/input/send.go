package input

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"progress-bar-3000/internal/config"
)

const SendProtocol = "progress-bar-3000/send-v1"
const MaxRequestBytes = 1024 * 1024
const MaxBatchEvents = 256

type sendRequest struct {
	Protocol string   `json:"protocol"`
	Events   []string `json:"events"`
}

type acknowledgement struct {
	Protocol string `json:"protocol"`
	OK       bool   `json:"ok"`
	Count    int    `json:"count"`
	Error    string `json:"error,omitempty"`
}

// Request groups event lines with an optional application acknowledgement.
// Reply must be called after applying the complete batch, before completion hooks.
type Request struct {
	Lines []string
	Reply func(error) error
}

type RequestSource interface {
	RunRequests(context.Context, func(Request) error) error
}

func ValidateSocketPath(path string) error {
	capacity := len(syscall.RawSockaddrUnix{}.Path)
	if len(path) >= capacity {
		return fmt.Errorf("socket path is %d bytes; the platform limit is %d bytes (%d-byte sun_path including the terminator); use a shorter directory such as /tmp", len(path), capacity-1, capacity)
	}
	if path == "" || strings.ContainsRune(path, 0) {
		return fmt.Errorf("socket path must be nonempty and contain no nul bytes")
	}
	return nil
}

// Send succeeds only after the renderer acknowledges applying every event.
// A missing acknowledgement is ambiguous: never automatically retry increments.
func Send(ctx context.Context, path string, lines []string) error {
	if err := ValidateSocketPath(path); err != nil {
		return err
	}
	if err := validateBatch(lines); err != nil {
		return err
	}
	data, err := json.Marshal(sendRequest{Protocol: SendProtocol, Events: lines})
	if err != nil {
		return err
	}
	if len(data)+1 > MaxRequestBytes {
		return fmt.Errorf("event batch exceeds the %d-byte limit", MaxRequestBytes)
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return fmt.Errorf("connect to renderer: %w", err)
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("send events (delivery may be incomplete): %w", err)
	}
	reader := bufio.NewReader(io.LimitReader(conn, 64*1024))
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("renderer did not acknowledge; events may already have applied, do not blindly retry: %w", err)
	}
	var ack acknowledgement
	if err := decodeStrict(line, &ack); err != nil {
		return fmt.Errorf("invalid renderer acknowledgement: %w", err)
	}
	if ack.Protocol != SendProtocol {
		return fmt.Errorf("invalid renderer acknowledgement protocol %q", ack.Protocol)
	}
	if !ack.OK {
		return fmt.Errorf("renderer rejected batch: %s", ack.Error)
	}
	if ack.Count != len(lines) || ack.Error != "" {
		return fmt.Errorf("invalid renderer acknowledgement: expected %d applied events, got %d", len(lines), ack.Count)
	}
	return nil
}

func validateBatch(lines []string) error {
	if len(lines) == 0 || len(lines) > MaxBatchEvents {
		return fmt.Errorf("send requires between 1 and %d event lines", MaxBatchEvents)
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" || strings.ContainsAny(line, "\r\n") {
			return fmt.Errorf("event %d must be one nonempty line", i+1)
		}
		if strings.HasPrefix(strings.TrimSpace(line), "{") && !json.Valid([]byte(line)) {
			return fmt.Errorf("event %d must contain exactly one json object", i+1)
		}
		if _, err := ParseLine(config.InputModeAuto, line); err != nil {
			return fmt.Errorf("event %d: %w", i+1, err)
		}
	}
	return nil
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected exactly one json object")
	}
	return nil
}

func decodeSendRequest(line string) ([]string, bool, error) {
	if !hasProtocolField(line) {
		return nil, false, nil
	}
	var request sendRequest
	if err := decodeStrict([]byte(line), &request); err != nil {
		return nil, true, err
	}
	if request.Protocol != SendProtocol {
		return nil, true, fmt.Errorf("unsupported send protocol %q", request.Protocol)
	}
	if err := validateBatch(request.Events); err != nil {
		return nil, true, err
	}
	return request.Events, true, nil
}

// inspect keys incrementally so a truncated envelope cannot become a raw event.
func hasProtocolField(line string) bool {
	decoder := json.NewDecoder(strings.NewReader(line))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return false
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return false
		}
		if key == "protocol" {
			return true
		}
		var ignored json.RawMessage
		if err := decoder.Decode(&ignored); err != nil {
			return false
		}
	}
	return false
}

func writeAcknowledgement(conn net.Conn, count int, err error) error {
	ack := acknowledgement{Protocol: SendProtocol, OK: err == nil, Count: count}
	if err != nil {
		ack.Error = err.Error()
		ack.Count = 0
	}
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	return json.NewEncoder(conn).Encode(ack)
}
