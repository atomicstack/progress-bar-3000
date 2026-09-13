package input

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PhasePlan keeps parent names and their optional one-level child plans together.
type PhasePlan struct {
	Names     []string
	Subphases map[int][]string
}

func (p *PhasePlan) UnmarshalJSON(data []byte) error {
	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	plan := PhasePlan{}
	if entries != nil {
		plan.Names = make([]string, 0, len(entries))
	}
	for i, entry := range entries {
		var name string
		if len(entry) > 0 && entry[0] == '"' {
			if err := json.Unmarshal(entry, &name); err != nil {
				return err
			}
		} else {
			var phase struct {
				Name      string   `json:"name"`
				Subphases []string `json:"subphases"`
			}
			decoder := json.NewDecoder(bytes.NewReader(entry))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&phase); err != nil {
				return fmt.Errorf("phase %d: %w", i, err)
			}
			if strings.TrimSpace(phase.Name) == "" {
				return fmt.Errorf("phase %d: name must not be empty", i)
			}
			if err := validateSubphases(phase.Subphases); err != nil {
				return fmt.Errorf("phase %d: %w", i, err)
			}
			name = phase.Name
			if len(phase.Subphases) > 0 {
				if plan.Subphases == nil {
					plan.Subphases = make(map[int][]string)
				}
				plan.Subphases[i] = phase.Subphases
			}
		}
		plan.Names = append(plan.Names, name)
	}
	*p = plan
	return nil
}

func validateSubphases(names []string) error {
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("subphase names must not be empty")
		}
	}
	return nil
}

func LoadPhaseFile(path string) (PhasePlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PhasePlan{}, fmt.Errorf("read phase file: %w", err)
	}

	if strings.EqualFold(filepath.Ext(path), ".json") {
		var phases PhasePlan
		if err := json.Unmarshal(data, &phases); err != nil {
			return PhasePlan{}, fmt.Errorf("parse phase json: %w", err)
		}
		return phases, nil
	}

	lines := strings.Split(string(data), "\n")
	phases := make([]string, 0, len(lines))
	for _, line := range lines {
		phase := strings.TrimSpace(line)
		if phase != "" {
			phases = append(phases, phase)
		}
	}
	return PhasePlan{Names: phases}, nil
}
