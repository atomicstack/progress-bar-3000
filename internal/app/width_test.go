package app

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/progress"
)

func barWidthModel(format string, width int) Model {
	return NewModel(config.Config{
		Format: format, Width: width,
		Style: config.StylePlain, BackgroundStyle: config.BackgroundStyleSpace,
		ColorMode: config.ColorModeNone, GradientStart: "#ffffff", GradientEnd: "#0087ff",
		FPS: 60, Lerp: 0.2,
	}, progress.State{Total: 100, Value: 50, DisplayValue: 50})
}

func TestDefaultBarWidthUsesNinetyPercentOfTerminal(t *testing.T) {
	for _, format := range []string{"%p", "%{progress}", "%{bar-only}"} {
		for _, tc := range []struct{ columns, want int }{
			{80, 72}, {120, 108}, {195, 175}, {79, 71}, {10, 9}, {1, 1}, {0, 72}, {-1, 72},
		} {
			t.Run(fmt.Sprintf("%s/%d", format, tc.columns), func(t *testing.T) {
				m := barWidthModel(format, 0)
				next, _ := m.Update(tea.WindowSizeMsg{Width: tc.columns, Height: 24})
				if got := len([]rune(stripANSI(next.(Model).View()))); got != tc.want {
					t.Fatalf("bar width = %d, want %d for %d terminal columns", got, tc.want, tc.columns)
				}
			})
		}
	}
}

func TestDefaultBarWidthBeforeTerminalSizeArrives(t *testing.T) {
	m := barWidthModel("%p", 0)
	if got := len([]rune(stripANSI(m.View()))); got != 72 {
		t.Fatalf("initial bar width = %d, want 72", got)
	}
}

func TestBarWidthRespondsToResizeAndHonorsOverrides(t *testing.T) {
	for _, tc := range []struct {
		name, format string
		width        int
		want         []int
	}{
		{"automatic", "%p", 0, []int{72, 108, 54}},
		{"explicit flag", "%p", 40, []int{40, 40, 40}},
		{"template overrides flag", "%7{bar-only}", 40, []int{7, 7, 7}},
		{"template overrides automatic", "%7{progress}", 0, []int{7, 7, 7}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := barWidthModel(tc.format, tc.width)
			for i, columns := range []int{80, 120, 60} {
				next, _ := m.Update(tea.WindowSizeMsg{Width: columns, Height: 24})
				m = next.(Model)
				if got := len([]rune(stripANSI(m.View()))); got != tc.want[i] {
					t.Fatalf("bar width after resize to %d = %d, want %d", columns, got, tc.want[i])
				}
			}
		})
	}
}
