// Ported from lib/walker/create.js.

package csstree

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// WalkAction is what a walk handler returns to steer the walk (upstream:
// undefined, this.break / walk.break or this.skip / walk.skip).
type WalkAction int

const (
	// WalkContinue goes on with the walk.
	WalkContinue WalkAction = iota
	// WalkBreak stops the walk (upstream break).
	WalkBreak
	// WalkSkip, returned by Enter, skips the children of the node and its
	// Leave (upstream skip).
	WalkSkip
)

// WalkContext is the `this` of upstream walk handlers: the walk root and
// the nearest enclosing node of each walk context (NodeConfig.WalkContext),
// nil outside one. Selector is the enclosing SelectorList (upstream's
// SelectorList has walkContext "selector"); Function is the enclosing
// Function, PseudoClassSelector or PseudoElementSelector.
type WalkContext struct {
	Root          Node
	Stylesheet    Node
	Atrule        Node
	AtrulePrelude Node
	Rule          Node
	Selector      Node
	Block         Node
	Declaration   Node
	Function      Node
}

// get returns context[name] for a walk context name.
func (c *WalkContext) get(name string) Node {
	switch name {
	case "stylesheet":
		return c.Stylesheet
	case "atrule":
		return c.Atrule
	case "atrulePrelude":
		return c.AtrulePrelude
	case "rule":
		return c.Rule
	case "selector":
		return c.Selector
	case "block":
		return c.Block
	case "declaration":
		return c.Declaration
	case "function":
		return c.Function
	}
	return nil
}

// set does context[name] = node for a walk context name; other names are
// ignored.
func (c *WalkContext) set(name string, node Node) {
	switch name {
	case "stylesheet":
		c.Stylesheet = node
	case "atrule":
		c.Atrule = node
	case "atrulePrelude":
		c.AtrulePrelude = node
	case "rule":
		c.Rule = node
	case "selector":
		c.Selector = node
	case "block":
		c.Block = node
	case "declaration":
		c.Declaration = node
	case "function":
		c.Function = node
	}
}

// WalkHandler is called for a node during a walk; item and list are the
// list item holding node and its list, nil for a node held by a node field
// (or the root).
type WalkHandler func(context *WalkContext, node Node, item *ListItem[Node], list *List[Node]) WalkAction

// WalkOptions configures Walker.Walk (upstream walk(ast, fn | options); a
// lone function is Enter).
type WalkOptions struct {
	// Enter is called before the children of a node are visited.
	Enter WalkHandler
	// Leave is called after the children of a node are visited.
	Leave WalkHandler
	// Reverse visits fields and list items in reverse order.
	Reverse bool
	// Visit, when set, calls Enter and Leave only for nodes of that type;
	// for "Atrule", "Rule" and "Declaration" only the node types that can
	// contain them are traversed. Their WalkBreak and WalkSkip still apply
	// (upstream drops the return value of a handler when visit is set).
	Visit string
}

func noopWalkHandler(*WalkContext, Node, *ListItem[Node], *List[Node]) WalkAction {
	return WalkContinue
}

func invokeForType(fn WalkHandler, nodeType string) WalkHandler {
	return func(context *WalkContext, node Node, item *ListItem[Node], list *List[Node]) WalkAction {
		if node.Type() == nodeType {
			return fn(context, node, item, list)
		}
		return WalkContinue
	}
}

// walkerType is upstream's { context, fields } for a node type.
type walkerType struct {
	context string
	fields  WalkFieldsFunc
}

// getWalkersFromStructure returns nil (upstream null) for a node type with
// no node or list fields. Upstream iterates the fields by name; here the
// node type's Walk function does, so a type without one is not descended
// into.
func getWalkersFromStructure(nodeType NodeConfig) *walkerType {
	walkers := 0

	for _, field := range nodeType.Structure {
		isWalker := false

		for _, fieldType := range field.Types {
			if fieldType.Kind == FieldNode || fieldType.Kind == FieldList {
				isWalker = true
			}
		}

		if isWalker {
			walkers++
		}
	}

	if walkers > 0 && nodeType.Walk != nil {
		return &walkerType{
			context: nodeType.WalkContext,
			fields:  nodeType.Walk,
		}
	}

	return nil
}

// getTypesFromConfig maps every node type of config to its walker (nil
// for a type with nothing to walk).
func getTypesFromConfig(config *Config) (map[string]*walkerType, error) {
	types := make(map[string]*walkerType)

	for name, nodeType := range config.Node {
		if nodeType.Structure == nil {
			//lint:ignore ST1005 upstream message
			return nil, fmt.Errorf("Missed `structure` field in `%s` node type definition", name)
		}

		types[name] = getWalkersFromStructure(nodeType)
	}

	return types, nil
}

// typeIterator walks the fields of a node with walkNode and walkList and
// reports whether the walk broke.
type typeIterator func(node Node, context *WalkContext, walkNode func(node Node) bool, walkList func(list *List[Node]) bool) bool

func createTypeIterator(config *walkerType, reverse bool) typeIterator {
	fields := config.fields
	contextName := config.context
	useContext := contextName != ""

	return func(node Node, context *WalkContext, walkNode func(node Node) bool, walkList func(list *List[Node]) bool) bool {
		var prevContextValue Node

		if useContext {
			prevContextValue = context.get(contextName)
			context.set(contextName, node)
		}

		if fields(node, reverse, walkNode, walkList) {
			return true
		}

		if useContext {
			context.set(contextName, prevContextValue)
		}

		return false
	}
}

func createFastTraveralMap(iterators map[string]typeIterator) map[string]map[string]typeIterator {
	pick := func(names ...string) map[string]typeIterator {
		result := make(map[string]typeIterator)
		for _, name := range names {
			if iterator, ok := iterators[name]; ok {
				result[name] = iterator
			}
		}
		return result
	}

	return map[string]map[string]typeIterator{
		"Atrule":      pick("StyleSheet", "Atrule", "Rule", "Block"),
		"Rule":        pick("StyleSheet", "Atrule", "Rule", "Block"),
		"Declaration": pick("StyleSheet", "Atrule", "Rule", "Block", "DeclarationList"),
	}
}

// Walker traverses ASTs of the node types of a config (upstream walk, the
// function createWalker returns, with its find, findLast and findAll
// properties). A Walker is safe for concurrent use; see Walk for the ASTs
// it walks.
type Walker struct {
	types                         map[string]*walkerType
	iteratorsNatural              map[string]typeIterator
	iteratorsReverse              map[string]typeIterator
	fastTraversalIteratorsNatural map[string]map[string]typeIterator
	fastTraversalIteratorsReverse map[string]map[string]typeIterator
}

// CreateWalker returns a walker for the node types of config (upstream
// createWalker). It fails when a node type has no Structure.
func CreateWalker(config *Config) (*Walker, error) {
	if config == nil {
		config = new(Config)
	}
	types, err := getTypesFromConfig(config)
	if err != nil {
		return nil, err
	}
	iteratorsNatural := make(map[string]typeIterator)
	iteratorsReverse := make(map[string]typeIterator)

	for name, walker := range types {
		if walker != nil {
			iteratorsNatural[name] = createTypeIterator(walker, false)
			iteratorsReverse[name] = createTypeIterator(walker, true)
		}
	}

	return &Walker{
		types:                         types,
		iteratorsNatural:              iteratorsNatural,
		iteratorsReverse:              iteratorsReverse,
		fastTraversalIteratorsNatural: createFastTraveralMap(iteratorsNatural),
		fastTraversalIteratorsReverse: createFastTraveralMap(iteratorsReverse),
	}, nil
}

// Walk traverses root depth-first, calling options.Enter before and
// options.Leave after the children of each node. It fails when neither
// handler is set or options.Visit names an unknown node type. Nil nodes
// (a nil root or nil list data) are not visited.
//
// The handlers may change the list holding the current node (insert or
// remove items, as List.ForEach allows), which is why Walk records its
// position in each list it walks: it must not run concurrently with any
// other use of the same AST, not even a read-only one such as Generate or
// Find.
func (w *Walker) Walk(root Node, options WalkOptions) error {
	return w.walk(root, options, true)
}

// walk is Walk; mutable is walkAST's.
func (w *Walker) walk(root Node, options WalkOptions, mutable bool) error {
	enter := WalkHandler(noopWalkHandler)
	leave := WalkHandler(noopWalkHandler)
	enterIsNoop := true
	leaveIsNoop := true
	iterators := w.iteratorsNatural

	if options.Enter != nil {
		enter = options.Enter
		enterIsNoop = false
	}
	if options.Leave != nil {
		leave = options.Leave
		leaveIsNoop = false
	}

	if options.Reverse {
		iterators = w.iteratorsReverse
	}

	if options.Visit != "" {
		if fastTraversal, ok := w.fastTraversalIteratorsNatural[options.Visit]; ok {
			if options.Reverse {
				iterators = w.fastTraversalIteratorsReverse[options.Visit]
			} else {
				iterators = fastTraversal
			}
		} else if _, ok := w.types[options.Visit]; !ok {
			names := make([]string, 0, len(w.types))
			for name := range w.types {
				names = append(names, name)
			}
			slices.Sort(names)
			//lint:ignore ST1005 upstream message
			return fmt.Errorf("Bad value `%s` for `visit` option (should be: %s)", options.Visit, strings.Join(names, ", "))
		}

		enter = invokeForType(enter, options.Visit)
		leave = invokeForType(leave, options.Visit)
		enterIsNoop = false
		leaveIsNoop = false
	}

	if enterIsNoop && leaveIsNoop {
		//lint:ignore ST1005 upstream message
		return errors.New("Neither `enter` nor `leave` walker handler is set or both aren't a function")
	}

	walkAST(root, enter, leave, iterators, options.Reverse, mutable)

	return nil
}

// walkAST is the traversal of upstream walk once the options are resolved.
// iterators walk the fields of nodes in reverse order when reverse is set.
// When mutable is set, lists are iterated with ForEach or ForEachRight, so
// that handlers may change the list being walked; otherwise they are only
// read, so that walks of one AST may run concurrently.
func walkAST(root Node, enter, leave WalkHandler, iterators map[string]typeIterator, reverse, mutable bool) {
	var walkNode func(node Node, item *ListItem[Node], list *List[Node]) bool
	context := &WalkContext{
		Root: root,
	}

	walkList := func(list *List[Node]) bool {
		switch {
		case mutable:
			broke := false
			visit := func(data Node, item *ListItem[Node]) {
				broke = broke || walkNode(data, item, list)
			}
			if reverse {
				list.ForEachRight(visit)
			} else {
				list.ForEach(visit)
			}
			return broke
		case reverse:
			for item := list.Tail(); item != nil; item = item.Prev {
				if walkNode(item.Data, item, list) {
					return true
				}
			}
		default:
			for item := list.Head(); item != nil; item = item.Next {
				if walkNode(item.Data, item, list) {
					return true
				}
			}
		}
		return false
	}
	walkField := func(node Node) bool {
		return walkNode(node, nil, nil)
	}

	walkNode = func(node Node, item *ListItem[Node], list *List[Node]) bool {
		if node == nil {
			return false
		}

		enterRet := enter(context, node, item, list)

		if enterRet == WalkBreak {
			return true
		}

		if enterRet == WalkSkip {
			return false
		}

		if iterator, ok := iterators[node.Type()]; ok {
			if iterator(node, context, walkField, walkList) {
				return true
			}
		}

		if leave(context, node, item, list) == WalkBreak {
			return true
		}

		return false
	}

	walkNode(root, nil, nil)
}

// Find returns the first node, in natural walk order, for which fn returns
// true, or nil. fn must not change the AST: Find, FindLast and FindAll only
// read it, so they may run concurrently with other reads of the same AST.
func (w *Walker) Find(ast Node, fn func(context *WalkContext, node Node, item *ListItem[Node], list *List[Node]) bool) Node {
	var found Node

	walkAST(ast, func(context *WalkContext, node Node, item *ListItem[Node], list *List[Node]) WalkAction {
		if fn(context, node, item, list) {
			found = node
			return WalkBreak
		}
		return WalkContinue
	}, noopWalkHandler, w.iteratorsNatural, false, false)

	return found
}

// FindLast returns the first node, in reverse walk order, for which fn
// returns true, or nil.
func (w *Walker) FindLast(ast Node, fn func(context *WalkContext, node Node, item *ListItem[Node], list *List[Node]) bool) Node {
	var found Node

	walkAST(ast, func(context *WalkContext, node Node, item *ListItem[Node], list *List[Node]) WalkAction {
		if fn(context, node, item, list) {
			found = node
			return WalkBreak
		}
		return WalkContinue
	}, noopWalkHandler, w.iteratorsReverse, true, false)

	return found
}

// FindAll returns every node, in natural walk order, for which fn returns
// true.
func (w *Walker) FindAll(ast Node, fn func(context *WalkContext, node Node, item *ListItem[Node], list *List[Node]) bool) []Node {
	var found []Node

	walkAST(ast, func(context *WalkContext, node Node, item *ListItem[Node], list *List[Node]) WalkAction {
		if fn(context, node, item, list) {
			found = append(found, node)
		}
		return WalkContinue
	}, noopWalkHandler, w.iteratorsNatural, false, false)

	return found
}
