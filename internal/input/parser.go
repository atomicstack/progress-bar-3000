package input

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"progress-bar-3000/internal/config"
)

func ParseLine(mode config.InputMode, line string) (Event, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return Event{}, nil
	}

	if strings.HasPrefix(trimmed, "{") {
		return parseJSONEvent(trimmed)
	}

	if strings.HasPrefix(trimmed, "@") {
		return parseControlLine(trimmed)
	}

	if mode == config.InputModeValue {
		value, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return Event{}, fmt.Errorf("parse value line: %w", err)
		}
		return Event{Kind: KindValue, Value: value}, nil
	}

	return Event{Kind: KindTick, Amount: 1, Label: trimmed}, nil
}

func parseControlLine(line string) (Event, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return Event{}, nil
	}

	command := strings.TrimPrefix(fields[0], "@")
	rest := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))

	switch command {
	case "tick":
		if rest == "" {
			return Event{Kind: KindTick, Amount: 1}, nil
		}
		amount, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			return Event{}, fmt.Errorf("parse @tick amount: %w", err)
		}
		return Event{Kind: KindTick, Amount: amount}, nil
	case "inc":
		amount, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			return Event{}, fmt.Errorf("parse @inc amount: %w", err)
		}
		return Event{Kind: KindIncrement, Amount: amount}, nil
	case "value":
		value, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			return Event{}, fmt.Errorf("parse @value amount: %w", err)
		}
		return Event{Kind: KindValue, Value: value}, nil
	case "set-total":
		total, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil {
			return Event{}, fmt.Errorf("parse @set-total amount: %w", err)
		}
		return Event{Kind: KindSetTotal, Total: total}, nil
	case "phase":
		index, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil {
			return Event{}, fmt.Errorf("parse @phase index: %w", err)
		}
		return Event{Kind: KindPhase, PhaseIndex: index}, nil
	case "phase-name":
		return Event{Kind: KindPhase, PhaseName: strings.TrimSpace(rest)}, nil
	case "set-subphases":
		names := []string{}
		if rest != "" {
			for _, part := range strings.Split(rest, ",") {
				names = append(names, strings.TrimSpace(part))
			}
		}
		if err := validateSubphases(names); err != nil {
			return Event{}, err
		}
		return Event{Kind: KindSetSubphases, Subphases: names}, nil
	case "subphase":
		index, err := strconv.Atoi(rest)
		if err != nil {
			return Event{}, fmt.Errorf("parse @subphase index: %w", err)
		}
		return Event{Kind: KindSubphase, SubphaseIndex: index}, nil
	case "subphase-name":
		if rest == "" {
			return Event{}, fmt.Errorf("parse @subphase-name: name is required")
		}
		return Event{Kind: KindSubphase, SubphaseName: rest}, nil
	case "on-complete":
		return Event{Kind: KindOnComplete, Command: rest}, nil
	case "label":
		return Event{Kind: KindLabel, Label: strings.TrimSpace(rest)}, nil
	case "meta":
		key, value, ok := strings.Cut(strings.TrimSpace(rest), "=")
		if !ok {
			return Event{}, fmt.Errorf("parse @meta: expected key=value")
		}
		return Event{Kind: KindMeta, Meta: map[string]string{strings.TrimSpace(key): strings.TrimSpace(value)}}, nil
	case "set-phases":
		if strings.TrimSpace(rest) == "" {
			return Event{Kind: KindReset, Phases: nil}, nil
		}
		parts := strings.Split(rest, ",")
		phases := make([]string, 0, len(parts))
		for _, part := range parts {
			phase := strings.TrimSpace(part)
			if phase != "" {
				phases = append(phases, phase)
			}
		}
		return Event{Kind: KindReset, Phases: phases}, nil
	case "reset":
		return Event{Kind: KindReset}, nil
	default:
		return Event{}, fmt.Errorf("unknown control command %q", command)
	}
}

type jsonEvent struct {
	Command   string            `json:"command"`
	Type      string            `json:"type"`
	Amount    float64           `json:"amount"`
	Value     float64           `json:"value"`
	Total     int               `json:"total"`
	Index     *int              `json:"index"`
	Name      string            `json:"name"`
	Label     string            `json:"label"`
	Meta      map[string]string `json:"meta"`
	Phases    *PhasePlan        `json:"phases"`
	Phase     string            `json:"phase"`
	Subphase  string            `json:"subphase"`
	Subphases []string          `json:"subphases"`
}

func parseJSONEvent(line string) (Event, error) {
	var payload jsonEvent
	decoder := json.NewDecoder(bytes.NewReader([]byte(line)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return Event{}, fmt.Errorf("parse json event: %w", err)
	}
	if payload.Phase != "" {
		switch payload.Type {
		case string(KindValue), string(KindTick), string(KindSubphase), string(KindSetSubphases):
		default:
			return Event{}, fmt.Errorf("json %s event does not accept phase; phase events use name or index", payload.Type)
		}
	}
	if payload.Subphase != "" {
		switch payload.Type {
		case string(KindValue), string(KindTick), string(KindPhase):
		default:
			return Event{}, fmt.Errorf("json %s event does not accept subphase; use a value, tick, or phase event", payload.Type)
		}
	}
	if payload.Subphases != nil && payload.Type != string(KindSetSubphases) {
		return Event{}, fmt.Errorf("json %s event does not accept subphases; use set_subphases or a structured reset plan", payload.Type)
	}

	switch payload.Type {
	case string(KindSetSubphases):
		if payload.Subphases == nil {
			return Event{}, fmt.Errorf("json set_subphases event requires a subphases array (use [] to clear)")
		}
		if err := validateSubphases(payload.Subphases); err != nil {
			return Event{}, err
		}
		return Event{Kind: KindSetSubphases, ParentPhase: payload.Phase, Subphases: payload.Subphases}, nil
	case string(KindSubphase):
		if payload.Name == "" && payload.Index == nil {
			return Event{}, fmt.Errorf("json subphase event requires name or index")
		}
		return Event{Kind: KindSubphase, ParentPhase: payload.Phase, SubphaseName: payload.Name, SubphaseIndex: indexOrZero(payload.Index)}, nil
	case string(KindOnComplete):
		return Event{Kind: KindOnComplete, Command: payload.Command}, nil
	case string(KindTick):
		amount := payload.Amount
		if amount == 0 {
			amount = 1
		}
		return Event{Kind: KindTick, Amount: amount, Label: payload.Label, ParentPhase: payload.Phase, SubphaseName: payload.Subphase}, nil
	case string(KindValue):
		return Event{Kind: KindValue, Value: payload.Value, ParentPhase: payload.Phase, SubphaseName: payload.Subphase}, nil
	case "set_total":
		return Event{Kind: KindSetTotal, Total: payload.Total}, nil
	case string(KindPhase):
		if payload.Phases != nil && len(payload.Phases.Names) > 0 {
			return Event{}, fmt.Errorf(`json phase event does not accept "phases"; use a reset event`)
		}
		return Event{Kind: KindPhase, PhaseIndex: indexOrZero(payload.Index), PhaseName: payload.Name, SubphaseName: payload.Subphase}, nil
	case string(KindLabel):
		return Event{Kind: KindLabel, Label: payload.Label}, nil
	case string(KindMeta):
		return Event{Kind: KindMeta, Meta: payload.Meta}, nil
	case string(KindReset):
		evt := Event{Kind: KindReset, Total: payload.Total}
		if payload.Phases != nil {
			evt.Phases = payload.Phases.Names
			evt.PhaseSubphases = payload.Phases.Subphases
		}
		return evt, nil
	default:
		return Event{}, fmt.Errorf("unknown json event type %q", payload.Type)
	}
}

func indexOrZero(index *int) int {
	if index == nil {
		return 0
	}
	return *index
}
