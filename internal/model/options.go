package model

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Variation struct {
	All    bool
	Pixels int
}

func ParseVariation(value string) (Variation, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "0" {
		return Variation{}, nil
	}
	if value == "all" || value == "*" {
		return Variation{All: true}, nil
	}
	pixels, err := strconv.Atoi(value)
	if err != nil || pixels < 0 {
		return Variation{}, fmt.Errorf("variation must be a non-negative integer or all")
	}
	return Variation{Pixels: pixels}, nil
}

type SecondsRange struct {
	Min float64
	Max float64
}

func ParseSecondsRange(value string) (*SecondsRange, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, "-")
	if len(parts) < 1 || len(parts) > 2 {
		return nil, fmt.Errorf("duration must be SECONDS or MIN-MAX")
	}
	minimum, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil, fmt.Errorf("duration must be SECONDS or MIN-MAX")
	}
	maximum := minimum
	if len(parts) == 2 {
		maximum, err = strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, fmt.Errorf("duration must be SECONDS or MIN-MAX")
		}
	}
	if !math.IsInf(minimum, 0) && !math.IsNaN(minimum) && !math.IsInf(maximum, 0) && !math.IsNaN(maximum) {
		if minimum < 0 || maximum < 0 {
			return nil, fmt.Errorf("duration must not be negative")
		}
		if minimum > maximum {
			return nil, fmt.Errorf("duration minimum must not exceed maximum")
		}
		return &SecondsRange{Min: minimum, Max: maximum}, nil
	}
	return nil, fmt.Errorf("duration must be finite")
}

var holdAliases = map[string]string{
	"control": "ctrl",
	"ctl":     "ctrl",
	"cmd":     "command",
	"windows": "win",
}

func ParseHold(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	seen := map[string]bool{}
	keys := []string{}
	for _, raw := range strings.Split(value, "+") {
		key := strings.ToLower(strings.TrimSpace(raw))
		if alias, ok := holdAliases[key]; ok {
			key = alias
		}
		if key == "" {
			return nil, fmt.Errorf("hold must be KEY or KEY+KEY")
		}
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys, nil
}

type PointOptions struct {
	Name           string
	Position       Position
	Variation      Variation
	Delay          *SecondsRange
	Hold           []string
	IncludeImports bool
}

type BoxOptions struct {
	Name           string
	Corners        [4]Position
	Variation      Variation
	Grid           *GridTarget
	Delay          *SecondsRange
	IncludeImports bool
}

type GridTarget struct {
	Rows    int
	Columns int
	Cell    int
}

func NewGridTarget(rows, columns, cell int) (*GridTarget, error) {
	if rows <= 0 || columns <= 0 {
		return nil, fmt.Errorf("grid rows and columns must be positive integers")
	}
	if columns > int(^uint(0)>>1)/rows {
		return nil, fmt.Errorf("grid is too large")
	}
	if cell < 0 || cell >= rows*columns {
		return nil, fmt.Errorf("grid cell must be between 0 and %d", rows*columns-1)
	}
	return &GridTarget{Rows: rows, Columns: columns, Cell: cell}, nil
}

type ImageOptions struct {
	Name          string
	PNG           []byte
	Variation     Variation
	Confidence    float64
	ReturnExists  bool
	WaitFor       bool
	NoClick       bool
	WaitUntilGone bool
	Timeout       float64
	Delay         *SecondsRange
	// Stall is retained for callers using the pre-delay option name.
	Stall          *SecondsRange
	ClickAll       bool
	Order          string
	Gap            *SecondsRange
	Hold           []string
	IncludeImports bool
}
