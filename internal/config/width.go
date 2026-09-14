package config

import (
	"fmt"
	"strconv"
)

// ParseWidth accepts a fixed column count, zero for the 90% default, or full
// for a rendered row that fits the viewport.
func ParseWidth(value string) (columns int, full bool, err error) {
	if value == "full" {
		return 0, true, nil
	}
	columns, err = strconv.Atoi(value)
	if err != nil || columns < 0 {
		return 0, false, fmt.Errorf("--width must be full or an integer greater than or equal to 0")
	}
	return columns, false, nil
}
