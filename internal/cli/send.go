package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"progress-bar-3000/internal/input"
)

func newSendCommand() *cobra.Command {
	var path string
	var jsonOnly bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use: "send [event ...]", Short: "send event lines and wait for the renderer to acknowledge applying them",
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				return fmt.Errorf("--socket-path is required")
			}
			if timeout <= 0 {
				return fmt.Errorf("--timeout must be positive")
			}
			lines := args
			if len(lines) == 0 {
				scanner := bufio.NewScanner(cmd.InOrStdin())
				scanner.Buffer(make([]byte, 4096), input.MaxRequestBytes+1)
				bytes := 0
				for scanner.Scan() {
					line := scanner.Text()
					bytes += len(line) + 1
					if bytes > input.MaxRequestBytes || len(lines) >= input.MaxBatchEvents {
						return fmt.Errorf("event batch exceeds send limits (%d events, %d bytes)", input.MaxBatchEvents, input.MaxRequestBytes)
					}
					lines = append(lines, line)
				}
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("read event lines: %w", err)
				}
			}
			if jsonOnly {
				for _, line := range lines {
					if !strings.HasPrefix(strings.TrimSpace(line), "{") || !json.Valid([]byte(line)) {
						return fmt.Errorf("--json requires one json event object per line")
					}
				}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			return input.Send(ctx, path, lines)
		},
	}
	cmd.Flags().StringVar(&path, "socket-path", "", "renderer unix socket path")
	cmd.Flags().BoolVar(&jsonOnly, "json", false, "require json event objects")
	cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "maximum time to connect, send and receive acknowledgement")
	return cmd
}
