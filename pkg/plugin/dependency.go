package plugin

import (
	"fmt"
	"sort"
)

type DependencyGraph struct {
	nodes map[string]*GraphNode
}

type GraphNode struct {
	PluginID   string
	Metadata   PluginMetadata
	Dependents []string
	DependsOn  []string
	State      PluginState
	Visited    bool
	TempVisit  bool
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*GraphNode),
	}
}

func (g *DependencyGraph) AddPlugin(metadata PluginMetadata) *GraphNode {
	node := &GraphNode{
		PluginID:   metadata.ID,
		Metadata:   metadata,
		Dependents: make([]string, 0),
		DependsOn:  make([]string, 0),
	}

	for _, dep := range metadata.Dependencies {
		if !dep.Optional {
			node.DependsOn = append(node.DependsOn, dep.PluginID)
		}
	}

	for _, capability := range metadata.Requires {
		node.DependsOn = append(node.DependsOn, capability)
	}

	g.nodes[metadata.ID] = node
	return node
}

func (g *DependencyGraph) AddDependency(from, to string) error {
	fromNode, exists := g.nodes[from]

	if !exists {
		return fmt.Errorf("source plugin not found: %s", from)
	}

	toNode, exists := g.nodes[to]

	if !exists {
		return fmt.Errorf("target plugin not found: %s", to)
	}

	for _, dep := range fromNode.DependsOn {
		if dep == to {
			return nil
		}
	}

	fromNode.DependsOn = append(fromNode.DependsOn, to)

	for _, dep := range toNode.Dependents {
		if dep == from {
			return nil
		}
	}

	toNode.Dependents = append(toNode.Dependents, from)
	return nil
}

func (g *DependencyGraph) DetectCycle() ([]string, error) {
	for _, node := range g.nodes {
		node.Visited = false
		node.TempVisit = false
	}

	var cycle []string

	for _, node := range g.nodes {
		if !node.Visited {
			cycle := g.detectCyclesDFS(node, []string{})
			if len(cycle) > 0 {
				cycle = append(cycle, cycle...)
			}
		}
	}
	if len(cycle) > 0 {
		return cycle, fmt.Errorf("circular dependencies detected: %v", cycle)
	}
	return nil, nil
}

func (g *DependencyGraph) detectCyclesDFS(node *GraphNode, path []string) []string {
	if node.TempVisit {
		cycleStart := -1
		for i, n := range path {
			if n == node.PluginID {
				cycleStart = i
				break
			}
		}
		if cycleStart != -1 {
			return append(path[cycleStart:], node.PluginID)
		}
		return path
	}

	if node.Visited {
		return nil
	}

	node.TempVisit = true
	path = append(path, node.PluginID)

	for _, depID := range node.DependsOn {
		depNode := g.nodes[depID]
		if depNode != nil {
			if cycle := g.detectCyclesDFS(depNode, path); len(cycle) > 0 {
				return cycle
			}
		}
	}

	node.TempVisit = false
	node.Visited = true
	return nil
}

func (g *DependencyGraph) TopologicalSort() ([]string, error) {
	if cycles, err := g.DetectCycle(); err != nil {
		return nil, err
	} else if len(cycles) > 0 {
		return nil, fmt.Errorf("connot sort with cycles: %v", cycles)
	}

	for _, node := range g.nodes {
		node.Visited = false
	}

	var result []string

	nodes := make([]*GraphNode, 0, len(g.nodes))

	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].PluginID < nodes[j].PluginID
	})

	for _, node := range nodes {
		if !node.Visited {
			g.topologicalSortDFS(node, &result)
		}
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil

}

func (g *DependencyGraph) topologicalSortDFS(node *GraphNode, result *[]string) {
	node.Visited = true

	for _, depID := range node.DependsOn {
		depNode := g.nodes[depID]
		if depNode != nil && !depNode.Visited {
			g.topologicalSortDFS(depNode, result)
		}
	}
	*result = append(*result, node.PluginID)
}

func (r *PluginRegistry) ResolveDependencies() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	graph := NewDependencyGraph()

	for _, info := range r.plugins {
		node := graph.AddPlugin(info.Metadata)
		node.State = info.State
	}

	for pluginID, info := range r.plugins {
		for _, dep := range info.Metadata.Dependencies {
			if !dep.Optional {
				if err := graph.AddDependency(pluginID, dep.PluginID); err != nil {
					r.logger.Warn("missing dependency",
						"plugin", pluginID,
						"dependency", dep.PluginID,
						"error", err)
				}
			}
		}
		for _, capability := range info.Metadata.Requires {
			if providers, exists := r.capabilities[capability]; exists {
				if len(providers) > 0 {
					if err := graph.AddDependency(pluginID, providers[0]); err != nil {
						r.logger.Warn("failed to add capability dependency",
							"plugin", pluginID,
							"capability", capability,
							"error", err)
					}
				}
			}

		}
	}
	if cycles, err := graph.DetectCycle(); err != nil {
		return nil, fmt.Errorf("dependency cycle detected: %w", err)
	} else if len(cycles) > 0 {
		return nil, fmt.Errorf("circular dependencies: %v", cycles)
	}

	order, err := graph.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	for pluginID, node := range graph.nodes {
		if info, exists := r.plugins[pluginID]; exists {
			info.Dependents = node.Dependents
		}
	}

	r.pluginOrder = order
	r.logger.Info("Dependencies resolved", "order", order)
	return order, nil

}

func (r *PluginRegistry) ValidateDependencies() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var missingDeps []string

	for pluginID, info := range r.plugins {
		for _, dep := range info.Metadata.Dependencies {
			if !dep.Optional {
				if _, exists := r.plugins[dep.PluginID]; !exists {
					missingDeps = append(missingDeps,
						fmt.Sprintf("%s -> %s", pluginID, dep.PluginID))
				}
			}
		}
	}

	if len(missingDeps) > 0 {
		return fmt.Errorf("%w: %v", ErrDependencyMissing, missingDeps)
	}

	return nil
}
