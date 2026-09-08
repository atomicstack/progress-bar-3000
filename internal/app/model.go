package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"progress-bar-3000/internal/config"
	fmtx "progress-bar-3000/internal/format"
	"progress-bar-3000/internal/input"
	"progress-bar-3000/internal/progress"
	"progress-bar-3000/internal/render"
)

type eventMsg struct {
	Event input.Event
	Now   time.Time
}

type frameMsg struct {
	Now time.Time
}

type doneMsg struct{}

type errMsg struct {
	Err error
}

type Model struct {
	cfg           config.Config
	template      fmtx.Template
	state         progress.State
	detail        []config.DetailKey
	detailFormats []fmtx.Template
	frame         int
	elapsed       float64
	// now is the wall-clock time of the latest frame; %{timer} is
	// rendered as now minus state.StartedAt.
	now time.Time
	err error
}

// detailRow renders one detail line below the bar from the current state.
// Centralised so that the formatting for each row lives in exactly one place.
var detailRenderers = map[config.DetailKey]func(progress.State) string{
	config.DetailPhase: func(s progress.State) string { return fmt.Sprintf("phase: %s", s.CurrentPhase()) },
	config.DetailValue: func(s progress.State) string { return fmt.Sprintf("value: %.0f/%d", s.Value, s.EffectiveTotal()) },
	config.DetailLabel: func(s progress.State) string { return fmt.Sprintf("label: %s", s.Label) },
}

func NewModel(cfg config.Config, initial progress.State) Model {
	tpl, _ := fmtx.Parse(cfg.Format)
	if len(initial.Phases) > 0 && initial.Value > 0 && initial.PhaseIndex == 0 {
		index := int(math.Floor(initial.Value))
		if index > 0 {
			index--
		}
		if index >= len(initial.Phases) {
			index = len(initial.Phases) - 1
		}
		initial.PhaseIndex = index
	}
	// Validate already ran, so any error here means programmer misuse.
	detail, _ := config.ParseDetail(cfg.Detail)
	detailFormats := make([]fmtx.Template, 0, len(cfg.DetailFormats))
	for _, raw := range cfg.DetailFormats {
		t, _ := fmtx.Parse(raw)
		detailFormats = append(detailFormats, t)
	}
	return Model{
		cfg:           cfg,
		template:      tpl,
		state:         initial,
		detail:        detail,
		detailFormats: detailFormats,
	}
}

func (m Model) Init() tea.Cmd {
	return nextFrameCmd(m.cfg.FPS)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Bubble Tea opens /dev/tty and puts it in raw mode when stdin is a
		// pipe, which means ctrl-c arrives as a key event instead of SIGINT.
		// Forward it to Quit explicitly so the bar is interruptible.
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	case eventMsg:
		previousDisplay := m.state.DisplayValue
		m.state.Apply(msg.Event, msg.Now)
		if affectsProgressValue(msg.Event.Kind) && msg.Event.Kind != input.KindReset {
			m.state.DisplayValue = previousDisplay
		}
		return m, nil
	case frameMsg:
		m.frame++
		// Seconds-since-epoch as the animation phase reference. This keeps
		// animations time-based (so they don't change speed with --fps) and
		// deterministic for tests that inject specific Now values.
		m.elapsed = float64(msg.Now.UnixNano()) / 1e9
		m.now = msg.Now
		gap := m.state.Value - m.state.DisplayValue
		if math.Abs(gap) < 0.001 {
			m.state.DisplayValue = m.state.Value
		} else {
			m.state.DisplayValue += gap * m.cfg.Lerp
		}
		return m, nextFrameCmd(m.cfg.FPS)
	case errMsg:
		m.err = msg.Err
		return m, tea.Quit
	case doneMsg:
		m.state.DisplayValue = m.state.Value
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m Model) View() string {
	resolver := resolver{cfg: m.cfg, state: m.state, elapsed: m.elapsed, now: m.now}
	rows := []string{m.template.Render(resolver)}
	for _, k := range m.detail {
		rows = append(rows, detailRenderers[k](m.state))
	}
	for _, t := range m.detailFormats {
		rows = append(rows, t.Render(resolver))
	}
	// Trailing newline shifts Bubble Tea's render area down by one empty row.
	// On graceful shutdown the renderer's EraseEntireLine targets that empty
	// row instead of the bar, so the bar (and any detail lines) stay visible.
	// Run() handles the --clear-on-exit case by erasing those rows itself.
	return strings.Join(rows, "\n") + "\n"
}

type resolver struct {
	cfg     config.Config
	state   progress.State
	elapsed float64
	now     time.Time
}

func (r resolver) Resolve(name string, width int) string {
	switch name {
	case "progress", "bar-only":
		barWidth := width
		if barWidth == 0 {
			if r.cfg.Width > 0 {
				barWidth = r.cfg.Width
			} else {
				barWidth = 20
			}
		}
		start, end := animatedGradient(r.cfg)
		pulse, shimmer, shift := animationState(r.cfg, r.elapsed)
		return render.RenderBar(render.Options{
			Width:           barWidth,
			Percent:         clampPercent(r.state.DisplayValue, r.state.EffectiveTotal()),
			Style:           render.Style(r.cfg.Style),
			BackgroundStyle: render.BackgroundStyle(r.cfg.BackgroundStyle),
			BackgroundRune:  r.cfg.BackgroundRune,
			Profile:         render.DetectProfile(string(r.cfg.ColorMode), render.ProfileTrueColor),
			GradientStart:   start,
			GradientEnd:     end,
			Pulse:           pulse,
			ShimmerPhase:    shimmer,
			GradientShift:   shift,
			ASCII:           r.cfg.ASCII,
		})
	case "percent":
		return fmt.Sprintf("%.0f%%", r.state.Percent())
	case "phase":
		return r.state.CurrentPhase()
	case "phase-index":
		return fmt.Sprintf("%d", r.state.PhaseIndex+1)
	case "phase-count":
		return fmt.Sprintf("%d", len(r.state.Phases))
	case "label":
		return r.state.Label
	case "value":
		return fmt.Sprintf("%.0f", r.state.Value)
	case "total":
		return fmt.Sprintf("%d", r.state.EffectiveTotal())
	case "rate":
		return fmt.Sprintf("%.2f/s", r.state.RatePerSecond())
	case "eta":
		return r.state.ETA().String()
	case "timer":
		return elapsedSince(r.state.StartedAt, r.now).String()
	default:
		if strings.HasPrefix(name, "meta:") {
			return r.state.Meta[strings.TrimPrefix(name, "meta:")]
		}
		return ""
	}
}

func animatedGradient(cfg config.Config) (render.RGB, render.RGB) {
	start, _ := render.ParseHexColor(cfg.GradientStart)
	end, _ := render.ParseHexColor(cfg.GradientEnd)
	return start, end
}

// animationState returns per-frame animation inputs derived from wall-clock
// elapsed seconds, so the visual pacing is independent of the configured FPS.
func animationState(cfg config.Config, elapsed float64) (pulse float64, shimmerPhase float64, gradientShift float64) {
	shimmerPhase = -1
	switch cfg.TintAnimation {
	case config.TintAnimationPulse:
		// 2.5s breath from 0 (natural colour) to 0.18 (brighter). Using
		// (1 - cos) rather than sin keeps the minimum at exactly the
		// original tint rather than leaving it permanently washed out.
		pulse = 0.09 * (1 - math.Cos(2*math.Pi*elapsed/2.5))
	case config.TintAnimationCycle:
		gradientShift = wrapUnit(elapsed / 4.0)
	case config.TintAnimationShimmer:
		shimmerPhase = wrapUnit(elapsed / 2.8)
	}
	return pulse, shimmerPhase, gradientShift
}

func wrapUnit(x float64) float64 {
	v := math.Mod(x, 1.0)
	if v < 0 {
		v += 1
	}
	return v
}

// elapsedSince is the whole-second duration between start and now for the
// %{timer} token. An unknown start (or a now that predates it, e.g. before
// the first frame has arrived) renders as 0s rather than a negative value.
func elapsedSince(start, now time.Time) time.Duration {
	if start.IsZero() {
		return 0
	}
	d := now.Sub(start).Truncate(time.Second)
	if d < 0 {
		return 0
	}
	return d
}

func clampPercent(value float64, total int) float64 {
	if total <= 0 {
		return 0
	}
	pct := value / float64(total)
	if pct < 0 {
		return 0
	}
	if pct > 1 {
		return 1
	}
	return pct
}

func stripANSI(in string) string {
	return render.StripANSI(in)
}

func nextFrameCmd(fps int) tea.Cmd {
	if fps <= 0 {
		fps = 60
	}
	return tea.Tick(time.Second/time.Duration(fps), func(now time.Time) tea.Msg {
		return frameMsg{Now: now}
	})
}

func affectsProgressValue(kind input.Kind) bool {
	switch kind {
	case input.KindTick, input.KindIncrement, input.KindValue:
		return true
	default:
		return false
	}
}
