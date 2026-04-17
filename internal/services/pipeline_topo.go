package services

import (
	"fmt"
	"sort"
)

// ---------------------------------------------------------------------------
// 拓撲排序（Kahn's algorithm）
// ---------------------------------------------------------------------------

func topoSortSteps(steps []StepDef) ([]StepDef, error) {
	byName := make(map[string]*StepDef, len(steps))
	inDegree := make(map[string]int, len(steps))
	for i := range steps {
		byName[steps[i].Name] = &steps[i]
		inDegree[steps[i].Name] = 0
	}

	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if _, ok := byName[dep]; !ok {
				return nil, fmt.Errorf("step %q depends on unknown step %q", s.Name, dep)
			}
			inDegree[s.Name]++
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	var sorted []StepDef
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		sorted = append(sorted, *byName[name])

		for _, s := range steps {
			for _, dep := range s.DependsOn {
				if dep == name {
					inDegree[s.Name]--
					if inDegree[s.Name] == 0 {
						queue = append(queue, s.Name)
						sort.Strings(queue)
					}
				}
			}
		}
	}

	if len(sorted) != len(steps) {
		return nil, fmt.Errorf("cycle detected in steps DAG")
	}
	return sorted, nil
}
