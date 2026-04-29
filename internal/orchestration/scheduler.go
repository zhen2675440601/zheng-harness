package orchestration

import (
	"fmt"
	"maps"
	"sort"
)

// DAGScheduler tracks dependency-aware readiness for a task decomposition.
type DAGScheduler struct {
	decomposition     TaskDecomposition
	readyQueue        []string
	dependencyCount   map[string]int
	dependents        map[string][]string
	subtasks          map[string]Subtask
	orderedSubtaskIDs []string
	completed         map[string]bool
	scheduled         map[string]bool
	queued            map[string]bool
}

// NewDAGScheduler constructs a dependency-aware scheduler from a decomposition.
func NewDAGScheduler(decomposition TaskDecomposition) (*DAGScheduler, error) {
	if err := decomposition.Validate(); err != nil {
		return nil, err
	}

	scheduler := &DAGScheduler{
		decomposition:     decomposition,
		dependencyCount:   make(map[string]int, len(decomposition.Subtasks)),
		dependents:        make(map[string][]string, len(decomposition.Subtasks)),
		subtasks:          make(map[string]Subtask, len(decomposition.Subtasks)),
		orderedSubtaskIDs: make([]string, 0, len(decomposition.Subtasks)),
		completed:         make(map[string]bool, len(decomposition.Subtasks)),
		scheduled:         make(map[string]bool, len(decomposition.Subtasks)),
		queued:            make(map[string]bool, len(decomposition.Subtasks)),
	}
	if err := scheduler.buildGraph(); err != nil {
		return nil, err
	}
	return scheduler, nil

}

// Schedule returns the next parallelizable batch of ready subtasks.
func (s *DAGScheduler) Schedule() []Subtask {
	if s == nil || len(s.readyQueue) == 0 {
		return nil
	}

	batchIDs := append([]string(nil), s.readyQueue...)
	s.readyQueue = nil
	for _, id := range batchIDs {
		s.queued[id] = false
		s.scheduled[id] = true
	}

	batch := make([]Subtask, 0, len(batchIDs))
	for _, id := range batchIDs {
		batch = append(batch, s.subtasks[id])
	}
	return batch
}

// MarkComplete records one completed subtask and returns any newly unblocked subtasks.
func (s *DAGScheduler) MarkComplete(subtaskID string) []Subtask {
	if s == nil {
		return nil
	}
	if _, ok := s.subtasks[subtaskID]; !ok {
		return nil
	}
	if s.completed[subtaskID] {
		return nil
	}

	s.completed[subtaskID] = true
	newlyReadyIDs := make([]string, 0)
	for _, dependentID := range s.dependents[subtaskID] {
		if s.completed[dependentID] {
			continue
		}
		if s.dependencyCount[dependentID] > 0 {
			s.dependencyCount[dependentID]--
		}
		if s.dependencyCount[dependentID] == 0 && !s.scheduled[dependentID] && !s.queued[dependentID] {
			s.readyQueue = append(s.readyQueue, dependentID)
			s.queued[dependentID] = true
			newlyReadyIDs = append(newlyReadyIDs, dependentID)
		}
	}

	s.sortReadyQueue()
	s.sortIDs(newlyReadyIDs)
	if len(newlyReadyIDs) == 0 {
		return nil
	}

	ready := make([]Subtask, 0, len(newlyReadyIDs))
	for _, id := range newlyReadyIDs {
		ready = append(ready, s.subtasks[id])
	}
	return ready
}

// TopologicalSort returns a deterministic execution order for the decomposition.
func (s *DAGScheduler) TopologicalSort() ([]Subtask, error) {
	if s == nil {
		return nil, nil
	}

	remaining := make(map[string]int, len(s.dependencyCount))
	maps.Copy(remaining, s.dependencyCount)

	readyIDs := make([]string, 0, len(remaining))
	for _, id := range s.orderedSubtaskIDs {
		if remaining[id] == 0 {
			readyIDs = append(readyIDs, id)
		}
	}
	s.sortIDs(readyIDs)

	ordered := make([]Subtask, 0, len(s.orderedSubtaskIDs))
	for len(readyIDs) > 0 {
		id := readyIDs[0]
		readyIDs = readyIDs[1:]
		ordered = append(ordered, s.subtasks[id])
		for _, dependentID := range s.dependents[id] {
			remaining[dependentID]--
			if remaining[dependentID] == 0 {
				readyIDs = append(readyIDs, dependentID)
			}
		}
		s.sortIDs(readyIDs)
	}

	if len(ordered) != len(s.orderedSubtaskIDs) {
		return nil, fmt.Errorf("task decomposition %q contains unschedulable dependencies", s.decomposition.TaskID)
	}
	return ordered, nil
}

func (s *DAGScheduler) buildGraph() error {
	index := make(map[string]int, len(s.decomposition.Subtasks))
	edges := make(map[string]map[string]struct{}, len(s.decomposition.Subtasks))

	for i, subtask := range s.decomposition.Subtasks {
		normalized := subtask.Normalize()
		s.subtasks[normalized.ID] = normalized
		s.orderedSubtaskIDs = append(s.orderedSubtaskIDs, normalized.ID)
		s.dependencyCount[normalized.ID] = 0
		index[normalized.ID] = i
	}

	addEdge := func(from, to string) error {
		if from == "" || to == "" {
			return fmt.Errorf("dependency endpoints must be non-empty")
		}
		if _, ok := s.subtasks[from]; !ok {
			return fmt.Errorf("dependency references unknown subtask %q", from)
		}
		if _, ok := s.subtasks[to]; !ok {
			return fmt.Errorf("dependency references unknown subtask %q", to)
		}
		if edges[from] == nil {
			edges[from] = make(map[string]struct{})
		}
		if _, exists := edges[from][to]; exists {
			return nil
		}
		edges[from][to] = struct{}{}
		s.dependents[from] = append(s.dependents[from], to)
		s.dependencyCount[to]++
		return nil
	}

	for _, subtask := range s.decomposition.Subtasks {
		for _, depID := range subtask.Dependencies {
			if err := addEdge(depID, subtask.ID); err != nil {
				return err
			}
		}
	}
	for _, edge := range s.decomposition.DAG {
		if edge.Type == DependencyTypeParallelWith {
			continue
		}
		if err := addEdge(edge.From, edge.To); err != nil {
			return err
		}
	}

	for id := range s.subtasks {
		if len(s.dependents[id]) > 1 {
			sort.SliceStable(s.dependents[id], func(i, j int) bool {
				return index[s.dependents[id][i]] < index[s.dependents[id][j]]
			})
		}
	}
	for _, id := range s.orderedSubtaskIDs {
		if s.dependencyCount[id] == 0 {
			s.readyQueue = append(s.readyQueue, id)
			s.queued[id] = true
		}
	}
	return nil
}

func (s *DAGScheduler) sortReadyQueue() {
	s.sortIDs(s.readyQueue)
}

func (s *DAGScheduler) sortIDs(ids []string) {
	if len(ids) < 2 {
		return
	}
	order := make(map[string]int, len(s.orderedSubtaskIDs))
	for i, id := range s.orderedSubtaskIDs {
		order[id] = i
	}
	sort.SliceStable(ids, func(i, j int) bool {
		return order[ids[i]] < order[ids[j]]
	})
}
