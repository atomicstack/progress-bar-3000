package input

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

type Source interface {
	Run(ctx context.Context, emit func(string) error) error
	Close() error
}

type readerSource struct {
	r io.Reader
}

func NewReaderSource(r io.Reader) Source {
	return &readerSource{r: r}
}

// NewNullSource returns a source that produces no events and exits only when
// its context is cancelled. Use when stdin is owned by the renderer (e.g. the
// TTY is being read by Bubble Tea for key input) so we don't steal bytes.
func NewNullSource() Source {
	return nullSource{}
}

type nullSource struct{}

func (nullSource) Run(ctx context.Context, _ func(string) error) error {
	<-ctx.Done()
	return ctx.Err()
}

func (nullSource) Close() error { return nil }

func (s *readerSource) Run(ctx context.Context, emit func(string) error) error {
	scanner := bufio.NewScanner(s.r)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := emit(scanner.Text()); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return ctx.Err()
}

func (s *readerSource) Close() error {
	return nil
}

type unixSocketSource struct {
	path       string
	listener   *net.UnixListener // immutable after construction; Close may race with AcceptUnix
	mu         sync.Mutex
	activeConn *net.UnixConn
	closeOnce  sync.Once
	closeErr   error
}

func NewUnixSocketSource(path string) (Source, error) {
	if _, err := os.Lstat(path); err == nil {
		return nil, fmt.Errorf("unix socket path %q already exists: %w", path, os.ErrExist)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("check unix socket path %q: %w", path, err)
	}

	addr := &net.UnixAddr{Name: path, Net: "unix"}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on unix socket %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("restrict unix socket permissions: %w", err)
	}

	return &unixSocketSource{
		path:     path,
		listener: listener,
	}, nil
}

func (s *unixSocketSource) Run(ctx context.Context, emit func(string) error) error {
	stop := make(chan struct{})
	defer close(stop)
	defer func() { _ = s.Close() }()

	go func() {
		select {
		case <-ctx.Done():
			_ = s.Close()
		case <-stop:
		}
	}()

	for {
		conn, err := s.listener.AcceptUnix()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if isClosedNetworkError(err) {
				return nil
			}
			return err
		}

		s.setActiveConn(conn)
		if err := ctx.Err(); err != nil {
			s.clearActiveConn(conn)
			return err
		}
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := emit(scanner.Text()); err != nil {
				return err
			}
		}
		s.clearActiveConn(conn)
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := scanner.Err(); err != nil && !isClosedNetworkError(err) {
			return err
		}
	}
}

func (s *unixSocketSource) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		conn := s.activeConn
		s.activeConn = nil
		listener := s.listener
		s.mu.Unlock()

		if conn != nil {
			_ = conn.Close()
		}
		if listener != nil {
			s.closeErr = listener.Close()
		}
		if removeErr := os.Remove(s.path); removeErr != nil && !os.IsNotExist(removeErr) && s.closeErr == nil {
			s.closeErr = removeErr
		}
	})
	return s.closeErr
}

func (s *unixSocketSource) setActiveConn(conn *net.UnixConn) {
	s.mu.Lock()
	s.activeConn = conn
	s.mu.Unlock()
}

func (s *unixSocketSource) clearActiveConn(conn *net.UnixConn) {
	s.mu.Lock()
	if s.activeConn == conn {
		s.activeConn = nil
	}
	s.mu.Unlock()
	_ = conn.Close()
}

func isClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	return strings.Contains(err.Error(), "use of closed network connection")
}
