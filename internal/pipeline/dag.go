package pipeline

import "fmt"

// DAG represents a directed acyclic graph of targets.
type DAG struct {
	nodes map[string]*dagNode
	order []string // topological order
}

type dagNode struct {
	name       string
	deps       []string
	dependants []string
}

// BuildDAG creates a DAG from the Torresmofile targets.
// If selectedTarget is non-empty, only include that target and its transitive dependencies.
func BuildDAG(tf *Torresmofile, selectedTarget string) (*DAG, error) {
	dag := &DAG{
		nodes: make(map[string]*dagNode, len(tf.Targets)),
	}

	// Build the node graph from targets.
	for _, t := range tf.Targets {
		dag.nodes[t.Name] = &dagNode{
			name: t.Name,
			deps: append([]string(nil), t.DependsOn...),
		}
	}

	// Wire up dependants (reverse edges).
	for _, node := range dag.nodes {
		for _, dep := range node.deps {
			depNode, ok := dag.nodes[dep]
			if !ok {
				return nil, fmt.Errorf("target %q depends on unknown target %q", node.name, dep)
			}
			depNode.dependants = append(depNode.dependants, node.name)
		}
	}

	// If a specific target is selected, prune to only that target and its
	// transitive dependencies.
	if selectedTarget != "" {
		if _, ok := dag.nodes[selectedTarget]; !ok {
			return nil, fmt.Errorf("target %q not found", selectedTarget)
		}

		keep := make(map[string]struct{})
		var walk func(name string)
		walk = func(name string) {
			if _, seen := keep[name]; seen {
				return
			}
			keep[name] = struct{}{}
			for _, dep := range dag.nodes[name].deps {
				walk(dep)
			}
		}
		walk(selectedTarget)

		for name := range dag.nodes {
			if _, ok := keep[name]; !ok {
				delete(dag.nodes, name)
			}
		}

		// Clean dependants to only include kept nodes.
		for _, node := range dag.nodes {
			filtered := node.dependants[:0]
			for _, d := range node.dependants {
				if _, ok := keep[d]; ok {
					filtered = append(filtered, d)
				}
			}
			node.dependants = filtered
		}
	}

	// Topological sort using Kahn's algorithm.
	inDegree := make(map[string]int, len(dag.nodes))
	for name, node := range dag.nodes {
		if _, exists := inDegree[name]; !exists {
			inDegree[name] = 0
		}
		for _, dep := range node.deps {
			if _, ok := dag.nodes[dep]; ok {
				inDegree[name]++
			}
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	// Sort the initial queue for deterministic ordering.
	sortStrings(queue)

	var order []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		node := dag.nodes[current]
		var nextReady []string
		for _, dep := range node.dependants {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				nextReady = append(nextReady, dep)
			}
		}
		sortStrings(nextReady)
		queue = append(queue, nextReady...)
	}

	if len(order) != len(dag.nodes) {
		return nil, fmt.Errorf("cycle detected in target dependencies")
	}

	dag.order = order
	return dag, nil
}

// Levels returns groups of targets that can run in parallel.
// Each level contains targets whose dependencies are all in previous levels.
func (d *DAG) Levels() [][]string {
	if len(d.order) == 0 {
		return nil
	}

	assigned := make(map[string]int, len(d.order))
	maxLevel := 0

	for _, name := range d.order {
		node := d.nodes[name]
		level := 0
		for _, dep := range node.deps {
			if _, ok := d.nodes[dep]; !ok {
				continue
			}
			if depLevel, ok := assigned[dep]; ok {
				if depLevel+1 > level {
					level = depLevel + 1
				}
			}
		}
		assigned[name] = level
		if level > maxLevel {
			maxLevel = level
		}
	}

	levels := make([][]string, maxLevel+1)
	for _, name := range d.order {
		l := assigned[name]
		levels[l] = append(levels[l], name)
	}

	return levels
}

// Order returns the flat topological order of targets.
func (d *DAG) Order() []string {
	return d.order
}

// sortStrings sorts a slice of strings in place using insertion sort.
// Avoids importing "sort" for a small utility.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
