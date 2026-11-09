package domain_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
)

func tasksDepending(deps map[int][]int, order ...int) []domain.Task {
	tasks := make([]domain.Task, 0, len(order))
	for _, n := range order {
		tasks = append(tasks, domain.Task{Number: n, DependsOn: deps[n]})
	}
	return tasks
}

func TestWaves(t *testing.T) {
	tests := []struct {
		name  string
		tasks []domain.Task
		want  [][]int
	}{
		{"no tasks", nil, [][]int{}},
		{"independent tasks share wave 1", tasksDepending(nil, 1, 2, 3), [][]int{{1, 2, 3}}},
		{"chain", tasksDepending(map[int][]int{2: {1}, 3: {2}}, 1, 2, 3), [][]int{{1}, {2}, {3}}},
		{"diamond", tasksDepending(map[int][]int{2: {1}, 3: {1}, 4: {2, 3}}, 1, 2, 3, 4), [][]int{{1}, {2, 3}, {4}}},
		{"task waits for the deepest dependency", tasksDepending(map[int][]int{2: {1}, 3: {1, 2}}, 1, 2, 3), [][]int{{1}, {2}, {3}}},
		{"dependent listed first", tasksDepending(map[int][]int{1: {2}}, 1, 2), [][]int{{2}, {1}}},
		{"unknown dependency is ignored", tasksDepending(map[int][]int{1: {9}}, 1), [][]int{{1}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.Waves(tc.tasks)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("waves = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWavesCycle(t *testing.T) {
	tests := []struct {
		name  string
		tasks []domain.Task
		want  string
	}{
		{"two tasks", tasksDepending(map[int][]int{1: {2}, 2: {1}}, 1, 2), "tasks 1, 2"},
		{"self dependency", tasksDepending(map[int][]int{3: {3}}, 3), "tasks 3"},
		{"cycle behind a free task", tasksDepending(map[int][]int{2: {3}, 3: {2}, 4: {1}}, 1, 2, 3, 4), "tasks 2, 3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.Waves(tc.tasks)
			if !errors.Is(err, domain.ErrDependencyCycle) {
				t.Fatalf("err = %v, want ErrDependencyCycle", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want it to contain %q", err, tc.want)
			}
			if got != nil {
				t.Errorf("waves = %v, want nil", got)
			}
		})
	}
}
