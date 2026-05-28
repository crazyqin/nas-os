// Package knowledgebase 提供个人知识库管理功能
package knowledgebase

import (
	"errors"
	"sync"
)

// Graph 知识图谱.
type Graph struct {
	mu    sync.RWMutex
	nodes map[string]*GraphNode
	edges []GraphEdge
}

// NewGraph 创建知识图谱.
func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[string]*GraphNode),
		edges: make([]GraphEdge, 0),
	}
}

// AddNode 添加节点.
func (g *Graph) AddNode(node GraphNode) error {
	if node.ID == "" {
		return errors.New("节点ID不能为空")
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.ID] = &node
	return nil
}

// RemoveNode 删除节点.
func (g *Graph) RemoveNode(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.nodes[id]; !ok {
		return errors.New("节点不存在")
	}

	// 删除相关边
	newEdges := make([]GraphEdge, 0)
	for _, e := range g.edges {
		if e.Source != id && e.Target != id {
			newEdges = append(newEdges, e)
		}
	}
	g.edges = newEdges

	delete(g.nodes, id)
	return nil
}

// GetNode 获取节点.
func (g *Graph) GetNode(id string) (*GraphNode, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.nodes[id]
	if !ok {
		return nil, errors.New("节点不存在")
	}
	return node, nil
}

// AddEdge 添加边.
func (g *Graph) AddEdge(edge GraphEdge) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.nodes[edge.Source]; !ok {
		return errors.New("源节点不存在")
	}
	if _, ok := g.nodes[edge.Target]; !ok {
		return errors.New("目标节点不存在")
	}

	g.edges = append(g.edges, edge)
	return nil
}

// RemoveEdge 删除边.
func (g *Graph) RemoveEdge(source, target string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	newEdges := make([]GraphEdge, 0)
	found := false
	for _, e := range g.edges {
		if e.Source == source && e.Target == target {
			found = true
			continue
		}
		newEdges = append(newEdges, e)
	}

	if !found {
		return errors.New("边不存在")
	}
	g.edges = newEdges
	return nil
}

// GetEdges 获取节点的所有边.
func (g *Graph) GetEdges(nodeID string) []GraphEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]GraphEdge, 0)
	for _, e := range g.edges {
		if e.Source == nodeID || e.Target == nodeID {
			result = append(result, e)
		}
	}
	return result
}

// GetNeighbors 获取邻居节点.
func (g *Graph) GetNeighbors(nodeID string) []*GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	neighborIDs := make(map[string]bool)
	for _, e := range g.edges {
		if e.Source == nodeID {
			neighborIDs[e.Target] = true
		}
		if e.Target == nodeID {
			neighborIDs[e.Source] = true
		}
	}

	result := make([]*GraphNode, 0)
	for id := range neighborIDs {
		if node, ok := g.nodes[id]; ok {
			result = append(result, node)
		}
	}
	return result
}

// GetGraphData 获取图谱数据.
func (g *Graph) GetGraphData() GraphData {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]GraphNode, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, *node)
	}

	edges := make([]GraphEdge, len(g.edges))
	copy(edges, g.edges)

	return GraphData{
		Nodes: nodes,
		Edges: edges,
	}
}

// FindPath 查找两个节点之间的路径.
func (g *Graph) FindPath(source, target string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[source]; !ok {
		return nil, errors.New("源节点不存在")
	}
	if _, ok := g.nodes[target]; !ok {
		return nil, errors.New("目标节点不存在")
	}

	// BFS 查找最短路径
	queue := []string{source}
	visited := make(map[string]bool)
	parent := make(map[string]string)
	visited[source] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == target {
			// 回溯路径
			path := make([]string, 0)
			for node := target; node != ""; node = parent[node] {
				path = append([]string{node}, path...)
			}
			return path, nil
		}

		for _, e := range g.edges {
			var next string
			if e.Source == current {
				next = e.Target
			} else if e.Target == current {
				next = e.Source
			}
			if next != "" && !visited[next] {
				visited[next] = true
				parent[next] = current
				queue = append(queue, next)
			}
		}
	}

	return nil, errors.New("路径不存在")
}

// GetBacklinks 获取反向链接.
func (g *Graph) GetBacklinks(nodeID string) []GraphEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]GraphEdge, 0)
	for _, e := range g.edges {
		if e.Target == nodeID {
			result = append(result, e)
		}
	}
	return result
}

// GetForwardLinks 获取正向链接.
func (g *Graph) GetForwardLinks(nodeID string) []GraphEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]GraphEdge, 0)
	for _, e := range g.edges {
		if e.Source == nodeID {
			result = append(result, e)
		}
	}
	return result
}

// BuildGraphFromDocs 从文档构建图谱.
func (g *Graph) BuildGraphFromDocs(docs []*Document, links []Link) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 清空现有数据
	g.nodes = make(map[string]*GraphNode)
	g.edges = make([]GraphEdge, 0)

	// 添加节点
	for _, doc := range docs {
		g.nodes[doc.ID] = &GraphNode{
			ID:    doc.ID,
			Title: doc.Title,
			Size:  len(doc.Links),
		}
	}

	// 添加边
	for _, link := range links {
		if _, ok := g.nodes[link.SourceID]; ok {
			if _, ok := g.nodes[link.TargetID]; ok {
				g.edges = append(g.edges, GraphEdge{
					Source: link.SourceID,
					Target: link.TargetID,
					Type:   link.Type,
					Weight: 1,
				})
			}
		}
	}
}

// GetConnectedComponents 获取连通分量.
func (g *Graph) GetConnectedComponents() [][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	components := make([][]string, 0)

	for nodeID := range g.nodes {
		if !visited[nodeID] {
			component := make([]string, 0)
			queue := []string{nodeID}

			for len(queue) > 0 {
				current := queue[0]
				queue = queue[1:]

				if visited[current] {
					continue
				}
				visited[current] = true
				component = append(component, current)

				for _, e := range g.edges {
					if e.Source == current && !visited[e.Target] {
						queue = append(queue, e.Target)
					}
					if e.Target == current && !visited[e.Source] {
						queue = append(queue, e.Source)
					}
				}
			}

			if len(component) > 0 {
				components = append(components, component)
			}
		}
	}

	return components
}

// GetNodeDegree 获取节点度数.
func (g *Graph) GetNodeDegree(nodeID string) (inDegree, outDegree int) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, e := range g.edges {
		if e.Source == nodeID {
			outDegree++
		}
		if e.Target == nodeID {
			inDegree++
		}
	}
	return
}

// GetMostConnectedNodes 获取连接最多的节点.
func (g *Graph) GetMostConnectedNodes(limit int) []GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	degree := make(map[string]int)
	for _, e := range g.edges {
		degree[e.Source]++
		degree[e.Target]++
	}

	type nodeDegree struct {
		node   GraphNode
		degree int
	}

	nodeDegrees := make([]nodeDegree, 0)
	for id, deg := range degree {
		if node, ok := g.nodes[id]; ok {
			nodeDegrees = append(nodeDegrees, nodeDegree{node: *node, degree: deg})
		}
	}

	// 排序
	for i := 0; i < len(nodeDegrees)-1; i++ {
		for j := i + 1; j < len(nodeDegrees); j++ {
			if nodeDegrees[j].degree > nodeDegrees[i].degree {
				nodeDegrees[i], nodeDegrees[j] = nodeDegrees[j], nodeDegrees[i]
			}
		}
	}

	result := make([]GraphNode, 0)
	for i, nd := range nodeDegrees {
		if limit > 0 && i >= limit {
			break
		}
		result = append(result, nd.node)
	}
	return result
}
