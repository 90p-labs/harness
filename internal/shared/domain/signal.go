package domain

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var ErrInvalidBreach = errors.New("invalid breach")

type Operator string

const (
	OperatorGreater      Operator = ">"
	OperatorGreaterEqual Operator = ">="
	OperatorLess         Operator = "<"
	OperatorLessEqual    Operator = "<="
)

var operators = []Operator{OperatorGreaterEqual, OperatorLessEqual, OperatorGreater, OperatorLess}

type Breach struct {
	Op    Operator
	Value float64
}

func ParseBreach(text string) (Breach, error) {
	text = strings.TrimSpace(strings.Trim(strings.TrimSpace(text), "`"))
	for _, op := range operators {
		rest, ok := strings.CutPrefix(text, string(op))
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return Breach{}, fmt.Errorf("%w: %q has no number after the operator", ErrInvalidBreach, text)
		}
		return Breach{Op: op, Value: value}, nil
	}
	return Breach{}, fmt.Errorf("%w: %q must start with >, >=, < or <=", ErrInvalidBreach, text)
}
