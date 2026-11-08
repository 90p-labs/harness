package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrDependencyCycle = errors.New("dependency cycle")

type CycleError struct {
	Tasks []int
}

func (e CycleError) Error() string {
	numbers := make([]string, len(e.Tasks))
	for i, n := range e.Tasks {
		numbers[i] = strconv.Itoa(n)
	}
	return fmt.Sprintf("%s among tasks %s", ErrDependencyCycle, strings.Join(numbers, ", "))
}

func (e CycleError) Unwrap() error {
	return ErrDependencyCycle
}

// A dependency that names no task is ignored; the task-depends rule reports it.
func Waves(tasks []Task) ([][]int, error) {
	known := make(map[int]bool, len(tasks))
	for _, t := range tasks {
		known[t.Number] = true
	}
	placed := make(map[int]bool, len(tasks))
	waves := [][]int{}
	remaining := tasks
	for len(remaining) > 0 {
		var wave []int
		var blocked []Task
		for _, t := range remaining {
			if dependenciesPlaced(t, known, placed) {
				wave = append(wave, t.Number)
			} else {
				blocked = append(blocked, t)
			}
		}
		if len(wave) == 0 {
			stuck := make([]int, len(blocked))
			for i, t := range blocked {
				stuck[i] = t.Number
			}
			return nil, CycleError{Tasks: stuck}
		}
		for _, n := range wave {
			placed[n] = true
		}
		waves = append(waves, wave)
		remaining = blocked
	}
	return waves, nil
}

func dependenciesPlaced(t Task, known, placed map[int]bool) bool {
	for _, d := range t.DependsOn {
		if known[d] && !placed[d] {
			return false
		}
	}
	return true
}
