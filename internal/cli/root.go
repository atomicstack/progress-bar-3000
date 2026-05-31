package cli

import (
	"fmt"
	"os"
	"unicode/utf8"

	"progress-bar-3000/internal/app"
	"progress-bar-3000/internal/config"

	"github.com/spf13/cobra"
)

func Execute() error {
	cfg := func(cfg config.Config) error {
		return app.Run(cfg, os.Stdin, os.Stdout, os.Stderr)
	}

	return NewRootCommand(cfg).Execute()
}

func NewRootCommand(run func(config.Config) error) *cobra.Command {
	cfg := config.Config{
		InputMode:       config.InputModeAuto,
		Format:          "%p %{percent} %{phase}",
		Style:           config.StyleGradientGranular,
		BackgroundStyle: config.BackgroundStyleSpace,
		ColorMode:       config.ColorModeAuto,
		GradientStart:   "#ffffff",
		GradientEnd:     "#0087ff",
		FPS:             60,
		Lerp:            0.18,
	}

	backgroundRune := ""

	cmd := &cobra.Command{
		Use:           "progress-bar-3000",
		Short:         "Render a terminal progress bar from flags or streamed input.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("bg-char") && utf8.RuneCountInString(backgroundRune) != 1 {
				return fmt.Errorf("--bg-char must be exactly one rune")
			}

			if backgroundRune != "" {
				cfg.BackgroundRune = []rune(backgroundRune)[0]
			}

			if err := cfg.Validate(); err != nil {
				return err
			}

			return run(cfg)
		},
	}

	cmd.Flags().IntVar(&cfg.Total, "total", cfg.Total, "total steps")
	cmd.Flags().IntVar(&cfg.Current, "current", cfg.Current, "current progress value")
	cmd.Flags().StringVar((*string)(&cfg.InputMode), "input-mode", string(cfg.InputMode), "input mode")
	cmd.Flags().StringVar(&cfg.Format, "format", cfg.Format, "format string")
	cmd.Flags().StringVar((*string)(&cfg.Style), "style", string(cfg.Style), "render style")
	cmd.Flags().StringVar((*string)(&cfg.BackgroundStyle), "bg-style", string(cfg.BackgroundStyle), "background style")
	cmd.Flags().StringVar(&backgroundRune, "bg-char", backgroundRune, "background character")
	cmd.Flags().StringVar((*string)(&cfg.ColorMode), "color-mode", string(cfg.ColorMode), "color mode")
	cmd.Flags().StringVar(&cfg.GradientStart, "gradient-start", cfg.GradientStart, "gradient start color")
	cmd.Flags().StringVar(&cfg.GradientEnd, "gradient-end", cfg.GradientEnd, "gradient end color")
	cmd.Flags().StringVar(&cfg.PhaseFile, "phase-file", cfg.PhaseFile, "phase file")
	cmd.Flags().StringVar(&cfg.Phase, "phase", cfg.Phase, "phase label")
	cmd.Flags().StringVar(&cfg.SocketPath, "socket-path", cfg.SocketPath, "unix socket path")
	cmd.Flags().IntVar(&cfg.Width, "width", cfg.Width, "output width")
	cmd.Flags().StringVar(&cfg.Detail, "detail", cfg.Detail, "show extra line(s) below the bar: comma-separated list of label, phase, value, or all (bare --detail = all)")
	cmd.Flags().Lookup("detail").NoOptDefVal = config.DetailAll
	cmd.Flags().StringArrayVar(&cfg.DetailFormats, "detail-format", cfg.DetailFormats, "additional detail row rendered from a format-string template (repeatable; same tokens as --format)")
	cmd.Flags().BoolVar(&cfg.ClearOnExit, "clear-on-exit", cfg.ClearOnExit, "erase the bar after completion instead of leaving it on screen")
	cmd.Flags().IntVar(&cfg.FPS, "fps", cfg.FPS, "frames per second")
	cmd.Flags().Float64Var(&cfg.Lerp, "lerp", cfg.Lerp, "lerp factor")
	cmd.Flags().StringVar((*string)(&cfg.TintAnimation), "tint-animation", string(cfg.TintAnimation), "tint animation: pulse, shimmer, or cycle (omit for none)")
	cmd.Flags().BoolVar(&cfg.ASCII, "ascii", cfg.ASCII, "use ascii characters")

	return cmd
}
