// Ported from lib/utils/clone.js.

package csstree

import (
	"fmt"
	"sync"
)

// defaultCloneTypes maps the node types of this package to their Clone
// functions.
var defaultCloneTypes = sync.OnceValue(func() map[string]CloneFunc {
	return cloneTypesFromConfig(&Config{Node: nodeConfigs()})
})

func cloneTypesFromConfig(config *Config) map[string]CloneFunc {
	types := make(map[string]CloneFunc)

	for name, nodeType := range config.Node {
		if nodeType.Clone != nil {
			types[name] = nodeType.Clone
		}
	}

	return types
}

// Clone returns a deep copy of node: new nodes, lists and locations
// throughout. A node of a type this package does not define is not copied
// (the copy shares it); use Syntax.Clone for node types added through
// Syntax.Fork.
func Clone(node Node) Node {
	return cloneWith(defaultCloneTypes())(node)
}

// cloneWith returns a clone function for the node types of types (upstream
// clone recurses into every plain object, array and List).
func cloneWith(types map[string]CloneFunc) func(node Node) Node {
	var clone func(node Node) Node

	clone = func(node Node) Node {
		if node == nil {
			return nil
		}

		if fn, ok := types[node.Type()]; ok {
			return fn(node, clone)
		}

		return node
	}

	return clone
}

// cloneAs clones node, a non-nil node field of type T. It panics when the
// Clone function of node's type returns another type, a bug in the
// CloneFunc of an extension that a nil field would hide.
func cloneAs[T Node](node T, clone func(Node) Node) T {
	cloned := clone(node)
	result, ok := cloned.(T)
	if !ok {
		panic(fmt.Sprintf("csstree: the Clone function of %s returned a %T, not a %T", node.Type(), cloned, node))
	}
	return result
}

// cloneLocation copies a node location (nil stays nil).
func cloneLocation(loc *Location) *Location {
	if loc == nil {
		return nil
	}

	result := *loc
	return &result
}

// cloneList returns a new list of clones of the nodes of list (nil stays
// nil).
func cloneList(list *List[Node], clone func(Node) Node) *List[Node] {
	if list == nil {
		return nil
	}

	result := new(List[Node])
	for data := range list.All() {
		result.AppendData(clone(data))
	}
	return result
}
