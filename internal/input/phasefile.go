package input

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadPhaseFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read phase file: %w", err)
	}

	if strings.EqualFold(filepath.Ext(path), ".json") {
		var phases []string
		if err := json.Unmarshal(data, &phases); err != nil {
			return nil, fmt.Errorf("parse phase json: %w", err)
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
	return phases, nil
}
