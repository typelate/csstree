// Ported from lib/definition-syntax/walk.js.

package definitionsyntax

import "errors"

// WalkOptions holds the walk handlers. At least one must be set.
type WalkOptions struct {
	// Enter is called for a node before its children.
	Enter func(node Node)
	// Leave is called for a node after its children.
	Leave func(node Node)
}

// ErrNilNode is the error Walk returns when it meets a nil node, such as the
// Term of a zero Multiplier.
var ErrNilNode = errors.New("definitionsyntax: nil node")

// Walk visits node and its descendants depth first. It returns an error when
// neither handler is set, ErrNilNode when it meets a nil node, and an
// "Unknown type" error for a *Range, which is only ever a Type's Opts.
func Walk(node Node, options WalkOptions) error {
	enter, leave := options.Enter, options.Leave

	if enter == nil && leave == nil {
		//lint:ignore ST1005 upstream message
		return errors.New("Neither `enter` nor `leave` walker handler is set or both aren't a function")
	}

	var walk func(node Node) error
	walk = func(node Node) error {
		if enter != nil {
			enter(node)
		}

		switch node := node.(type) {
		case *Group:
			for _, term := range node.Terms {
				if err := walk(term); err != nil {
					return err
				}
			}

		case *Multiplier:
			if err := walk(node.Term); err != nil {
				return err
			}

		case *Boolean:
			if err := walk(node.Term); err != nil {
				return err
			}

		case *Type, *Property, *Keyword, *AtKeyword, *Function, *String, *Token, *Comma:

		case nil:
			return ErrNilNode

		default:
			return errors.New("Unknown type: " + node.Type()) // upstream message
		}

		if leave != nil {
			leave(node)
		}
		return nil
	}

	return walk(node)
}
