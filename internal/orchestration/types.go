package orchestration

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// SubtaskStatus represents the execution lifecycle of a decomposed task unit.
type SubtaskStatus string

const (
	SubtaskStatusPending   SubtaskStatus = "pending"
	SubtaskStatusRunning   SubtaskStatus = "running"
	SubtaskStatusCompleted SubtaskStatus = "completed"
	SubtaskStatusFailed    SubtaskStatus = "failed"
)

// Normalize returns a deterministic supported status.
func (s SubtaskStatus) Normalize() SubtaskStatus {
	switch s {
	case SubtaskStatusPending, SubtaskStatusRunning, SubtaskStatusCompleted, SubtaskStatusFailed:
		return s
	default:
		return SubtaskStatusPending
	}
}

// IsValid reports whether the status is explicitly supported.
func (s SubtaskStatus) IsValid() bool {
	return s == SubtaskStatusPending || s == SubtaskStatusRunning || s == SubtaskStatusCompleted || s == SubtaskStatusFailed
}

// DependencyType captures how two subtasks relate inside a decomposition graph.
type DependencyType string

const (
	DependencyTypeDependsOn    DependencyType = "depends-on"
	DependencyTypeSequential   DependencyType = "sequential"
	DependencyTypeParallelWith DependencyType = "parallel-with"
)

// IsValid reports whether the dependency type is supported.
func (d DependencyType) IsValid() bool {
	return d == DependencyTypeDependsOn || d == DependencyTypeSequential || d == DependencyTypeParallelWith
}

// Dependency describes a relationship between two subtasks.
// For directed relationships, From must happen before To.
type Dependency struct {
	From string         `json:"from"`
	To   string         `json:"to"`
	Type DependencyType `json:"type"`
}

// Validate checks the structural integrity of a dependency edge.
func (d Dependency) Validate(validIDs map[string]struct{}) error {
	var errs []error
	if d.From == "" {
		errs = append(errs, errors.New("dependency from is required"))
	}
	if d.To == "" {
		errs = append(errs, errors.New("dependency to is required"))
	}
	if !d.Type.IsValid() {
		errs = append(errs, fmt.Errorf("dependency type %q is not supported", d.Type))
	}
	if d.From != "" {
		if _, ok := validIDs[d.From]; !ok {
			errs = append(errs, fmt.Errorf("dependency references unknown subtask %q", d.From))
		}
	}
	if d.To != "" {
		if _, ok := validIDs[d.To]; !ok {
			errs = append(errs, fmt.Errorf("dependency references unknown subtask %q", d.To))
		}
	}
	if d.From != "" && d.From == d.To {
		errs = append(errs, fmt.Errorf("dependency cannot reference itself %q", d.From))
	}
	return errors.Join(errs...)
}

// Subtask is the smallest planning unit in a decomposition.
type Subtask struct {
	ID             string        `json:"id"`
	Description    string        `json:"description"`
	Input          string        `json:"input,omitempty"`
	ExpectedOutput string        `json:"expected_output,omitempty"`
	Dependencies   []string      `json:"dependencies,omitempty"`
	Status         SubtaskStatus `json:"status"`
}

// Normalize applies backward-compatible defaults to a subtask.
func (s Subtask) Normalize() Subtask {
	s.Status = s.Status.Normalize()
	if len(s.Dependencies) == 0 {
		s.Dependencies = nil
	}
	return s
}

// MarshalJSON preserves normalized status values for persistence.
func (s Subtask) MarshalJSON() ([]byte, error) {
	type subtaskJSON Subtask
	return json.Marshal(subtaskJSON(s.Normalize()))
}

// UnmarshalJSON restores a subtask with normalized defaults.
func (s *Subtask) UnmarshalJSON(data []byte) error {
	type subtaskJSON Subtask
	var decoded subtaskJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*s = Subtask(decoded).Normalize()
	return nil
}

// Validate checks the structural integrity of a standalone subtask.
func (s Subtask) Validate() error {
	validIDs := map[string]struct{}{}
	if s.ID != "" {
		validIDs[s.ID] = struct{}{}
	}
	for _, depID := range s.Dependencies {
		if depID != "" {
			validIDs[depID] = struct{}{}
		}
	}
	return s.validate(validIDs)
}

func (s Subtask) validate(validIDs map[string]struct{}) error {
	var errs []error
	if s.ID == "" {
		errs = append(errs, errors.New("subtask id is required"))
	}
	if s.Description == "" {
		errs = append(errs, fmt.Errorf("subtask %q description is required", s.ID))
	}
	if !s.Status.IsValid() {
		errs = append(errs, fmt.Errorf("subtask %q has unsupported status %q", s.ID, s.Status))
	}
	seenDeps := make(map[string]struct{}, len(s.Dependencies))
	for _, depID := range s.Dependencies {
		if depID == "" {
			errs = append(errs, fmt.Errorf("subtask %q has empty dependency id", s.ID))
			continue
		}
		if depID == s.ID {
			errs = append(errs, fmt.Errorf("subtask %q cannot depend on itself", s.ID))
		}
		if _, dup := seenDeps[depID]; dup {
			errs = append(errs, fmt.Errorf("subtask %q has duplicate dependency %q", s.ID, depID))
			continue
		}
		seenDeps[depID] = struct{}{}
		if _, ok := validIDs[depID]; !ok {
			errs = append(errs, fmt.Errorf("subtask %q references unknown dependency %q", s.ID, depID))
		}
	}
	return errors.Join(errs...)
}

// TaskDecomposition stores a validated multi-subtask plan plus graph metadata.
type TaskDecomposition struct {
	TaskID   string            `json:"task_id"`
	Subtasks []Subtask         `json:"subtasks"`
	DAG      []Dependency      `json:"dag,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate checks identifier integrity, dependency references, and cycle safety.
func (d TaskDecomposition) Validate() error {
	var errs []error
	if d.TaskID == "" {
		errs = append(errs, errors.New("task decomposition task_id is required"))
	}
	if len(d.Subtasks) == 0 {
		errs = append(errs, errors.New("task decomposition requires at least one subtask"))
	}

	validIDs := make(map[string]struct{}, len(d.Subtasks))
	for _, subtask := range d.Subtasks {
		if subtask.ID == "" {
			continue
		}
		if _, exists := validIDs[subtask.ID]; exists {
			errs = append(errs, fmt.Errorf("duplicate subtask id %q", subtask.ID))
			continue
		}
		validIDs[subtask.ID] = struct{}{}
	}

	for _, subtask := range d.Subtasks {
		if err := subtask.validate(validIDs); err != nil {
			errs = append(errs, err)
		}
	}
	for _, edge := range d.DAG {
		if err := edge.Validate(validIDs); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if cycle := d.findCycle(); len(cycle) > 0 {
		return fmt.Errorf("task decomposition contains circular dependency: %s", formatCycle(cycle))
	}
	return nil
}

func (d TaskDecomposition) findCycle() []string {
	adjacency := make(map[string][]string, len(d.Subtasks))
	for _, subtask := range d.Subtasks {
		adjacency[subtask.ID] = append(adjacency[subtask.ID], nil...)
		for _, depID := range subtask.Dependencies {
			adjacency[depID] = append(adjacency[depID], subtask.ID)
		}
	}
	for _, edge := range d.DAG {
		if edge.Type == DependencyTypeParallelWith {
			continue
		}
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
	}

	keys := make([]string, 0, len(adjacency))
	for id := range adjacency {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	visited := make(map[string]bool, len(adjacency))
	inStack := make(map[string]bool, len(adjacency))
	path := make([]string, 0, len(adjacency))

	for _, id := range keys {
		if visited[id] {
			continue
		}
		if cycle := walkCycle(id, adjacency, visited, inStack, &path); len(cycle) > 0 {
			return cycle
		}
	}
	return nil
}

func walkCycle(node string, adjacency map[string][]string, visited, inStack map[string]bool, path *[]string) []string {
	visited[node] = true
	inStack[node] = true
	*path = append(*path, node)

	for _, next := range adjacency[node] {
		if !visited[next] {
			if cycle := walkCycle(next, adjacency, visited, inStack, path); len(cycle) > 0 {
				return cycle
			}
			continue
		}
		if inStack[next] {
			start := 0
			for i, current := range *path {
				if current == next {
					start = i
					break
				}
			}
			cycle := append([]string(nil), (*path)[start:]...)
			cycle = append(cycle, next)
			return cycle
		}
	}

	inStack[node] = false
	*path = (*path)[:len(*path)-1]
	return nil
}

func formatCycle(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(cycle[0])
	for i := 1; i < len(cycle); i++ {
		builder.WriteString(" -> ")
		builder.WriteString(cycle[i])
	}
	return builder.String()
}
