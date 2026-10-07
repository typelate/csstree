// Ported from lib/utils/List.js.

package csstree

import (
	"encoding/json"
	"errors"
	"iter"
)

//
//                              list
//                            ┌──────┐
//             ┌──────────────┼─head │
//             │              │ tail─┼──────────────┐
//             │              └──────┘              │
//             ▼                                    ▼
//            item        item        item        item
//          ┌──────┐    ┌──────┐    ┌──────┐    ┌──────┐
//  null ◀──┼─prev │◀───┼─prev │◀───┼─prev │◀───┼─prev │
//          │ next─┼───▶│ next─┼───▶│ next─┼───▶│ next─┼──▶ null
//          ├──────┤    ├──────┤    ├──────┤    ├──────┤
//          │ data │    │ data │    │ data │    │ data │
//          └──────┘    └──────┘    └──────┘    └──────┘
//

// The values List methods panic with when they are given an item of
// another list (upstream throws for two of these programming errors; the
// owner check catches the cases upstream silently corrupts).
var (
	errBeforeNotInList = errors.New("csstree: list: before item does not belong to the list")
	errItemNotInList   = errors.New("csstree: list: item does not belong to the list")
	errItemInList      = errors.New("csstree: list: item is already in a list")
	errListIntoItself  = errors.New("csstree: list: cannot insert a list into itself")
)

// ListItem is one link of a List. Prev and Next are nil at the ends; they
// are exported for traversal and must not be set directly.
type ListItem[T any] struct {
	Prev *ListItem[T]
	Next *ListItem[T]
	Data T

	// list is the list the item is linked into, nil when unlinked.
	list *List[T]
}

// listCursor records where an in-progress iteration continues, so that
// items can be inserted or removed while the list is being iterated.
// Cursors of nested iterations form a stack through the cursor field.
type listCursor[T any] struct {
	prev   *ListItem[T]
	next   *ListItem[T]
	cursor *listCursor[T]
}

// List is a doubly linked list. ForEach and ForEachRight tolerate their
// callback inserting or removing items of the list being iterated; All and
// Backward do not, and do not write to the list, so any number of
// goroutines may range over a list that nobody modifies.
//
// An item belongs to at most one list. The methods that take an item panic
// when it belongs to another list (Remove, Replace, the before argument of
// Insert) or is already in a list (the item Insert, Prepend and Append
// link): such misuse would otherwise corrupt both lists.
//
// The zero value is an empty list ready to use, and a nil *List reads as
// an empty list (Len, IsEmpty, First, Last, Head, Tail, All and Backward),
// so a hand-built node with nil children generates, walks and clones as
// one without children. ForEach and ForEachRight
// update the list's internal cursor stack, so they, like every mutating
// method, must not run concurrently with other uses of the list.
type List[T any] struct {
	head   *ListItem[T]
	tail   *ListItem[T]
	len    int
	cursor *listCursor[T]

	// releasedCursors is a free list of cursors for reuse.
	releasedCursors *listCursor[T]
}

// NewListItem returns an unlinked item holding data (upstream
// List.createItem).
//
// Type inference picks T from data, so for a List[Node] holding an
// *Identifier write NewListItem[Node].
func NewListItem[T any](data T) *ListItem[T] {
	return &ListItem[T]{Data: data}
}

// cursor helpers
func (l *List[T]) allocateCursor(prev, next *ListItem[T]) *listCursor[T] {
	var cursor *listCursor[T]

	if l.releasedCursors != nil {
		cursor = l.releasedCursors
		l.releasedCursors = l.releasedCursors.cursor
		cursor.prev = prev
		cursor.next = next
		cursor.cursor = l.cursor
	} else {
		cursor = &listCursor[T]{
			prev:   prev,
			next:   next,
			cursor: l.cursor,
		}
	}

	l.cursor = cursor

	return cursor
}

func (l *List[T]) releaseCursor() {
	cursor := l.cursor

	l.cursor = cursor.cursor
	cursor.prev = nil
	cursor.next = nil
	cursor.cursor = l.releasedCursors
	l.releasedCursors = cursor
}

func (l *List[T]) updateCursors(prevOld, prevNew, nextOld, nextNew *ListItem[T]) {
	cursor := l.cursor

	for cursor != nil {
		if cursor.prev == prevOld {
			cursor.prev = prevNew
		}

		if cursor.next == nextOld {
			cursor.next = nextNew
		}

		cursor = cursor.cursor
	}
}

// All returns an iterator over the data of l from head to tail
// (upstream [Symbol.iterator]). It only reads the list; removing the
// current item during the iteration ends it (use ForEach to mutate).
func (l *List[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := l.Head(); item != nil; item = item.Next {
			if !yield(item.Data) {
				return
			}
		}
	}
}

// Backward returns an iterator over the data of l from tail to head. Like
// All, it only reads the list.
func (l *List[T]) Backward() iter.Seq[T] {
	return func(yield func(T) bool) {
		for item := l.Tail(); item != nil; item = item.Prev {
			if !yield(item.Data) {
				return
			}
		}
	}
}

// Head returns the first item, or nil when l is empty.
func (l *List[T]) Head() *ListItem[T] {
	if l == nil {
		return nil
	}
	return l.head
}

// Tail returns the last item, or nil when l is empty.
func (l *List[T]) Tail() *ListItem[T] {
	if l == nil {
		return nil
	}
	return l.tail
}

// Len returns the number of items of l.
func (l *List[T]) Len() int {
	if l == nil {
		return 0
	}
	return l.len
}

// IsEmpty reports whether l has no items.
func (l *List[T]) IsEmpty() bool {
	return l.Head() == nil
}

// First returns the data of the head item, or the zero value of T (nil for
// List[Node]) when l is empty.
func (l *List[T]) First() T {
	var data T
	if head := l.Head(); head != nil {
		data = head.Data
	}
	return data
}

// Last returns the data of the tail item, or the zero value of T (nil for
// List[Node]) when l is empty.
func (l *List[T]) Last() T {
	var data T
	if tail := l.Tail(); tail != nil {
		data = tail.Data
	}
	return data
}

// MarshalJSON encodes l as a JSON array of its data (upstream toJSON).
func (l *List[T]) MarshalJSON() ([]byte, error) {
	array := make([]T, 0, l.Len())
	for data := range l.All() {
		array = append(array, data)
	}
	return json.Marshal(array)
}

// ForEach calls fn for each item from head to tail. fn may insert or remove
// items of l: items removed before they are reached are skipped and items
// inserted ahead of the current one are visited.
func (l *List[T]) ForEach(fn func(data T, item *ListItem[T])) {
	// push cursor
	cursor := l.allocateCursor(nil, l.head)

	for cursor.next != nil {
		item := cursor.next
		cursor.next = item.Next
		fn(item.Data, item)
	}

	// pop cursor
	l.releaseCursor()
}

// ForEachRight is ForEach from tail to head.
func (l *List[T]) ForEachRight(fn func(data T, item *ListItem[T])) {
	// push cursor
	cursor := l.allocateCursor(l.tail, nil)

	for cursor.prev != nil {
		item := cursor.prev
		cursor.prev = item.Prev
		fn(item.Data, item)
	}

	// pop cursor
	l.releaseCursor()
}

// mutation

// Clear removes every item of l. An iteration of l in progress ends after
// the current item.
func (l *List[T]) Clear() {
	for cursor := l.cursor; cursor != nil; cursor = cursor.cursor {
		cursor.prev = nil
		cursor.next = nil
	}

	for item := l.head; item != nil; {
		next := item.Next
		item.Prev = nil
		item.Next = nil
		item.list = nil
		item = next
	}

	l.head = nil
	l.tail = nil
	l.len = 0
}

// Copy returns a new list with new items holding the same data.
func (l *List[T]) Copy() *List[T] {
	result := new(List[T])

	for data := range l.All() {
		result.AppendData(data)
	}

	return result
}

// Prepend links item before the head and returns l. It panics if item is
// already in a list.
func (l *List[T]) Prepend(item *ListItem[T]) *List[T] {
	l.link(item)

	//      head
	//    ^
	// item
	l.updateCursors(nil, item, l.head, item)

	// insert to the beginning of the list
	if l.head != nil {
		// new item <- first item
		l.head.Prev = item
		// new item -> first item
		item.Next = l.head
	} else {
		// if list has no head, then it also has no tail
		// in this case tail points to the new item
		l.tail = item
	}

	// head always points to new item
	l.head = item
	return l
}

// PrependData links a new item holding data before the head and returns l.
func (l *List[T]) PrependData(data T) *List[T] {
	return l.Prepend(NewListItem(data))
}

// Append links item after the tail and returns l. It panics if item is
// already in a list.
func (l *List[T]) Append(item *ListItem[T]) *List[T] {
	return l.Insert(item, nil)
}

// AppendData links a new item holding data after the tail and returns l.
func (l *List[T]) AppendData(data T) *List[T] {
	return l.Insert(NewListItem(data), nil)
}

// link claims item for l, panicking if it is already in a list.
func (l *List[T]) link(item *ListItem[T]) {
	if item.list != nil {
		panic(errItemInList)
	}
	item.list = l
	l.len++
}

// Insert links item before the item before, or after the tail when before
// is nil, and returns l. It panics if item is already in a list or before
// does not belong to l.
func (l *List[T]) Insert(item, before *ListItem[T]) *List[T] {
	if before != nil && before.list != l {
		panic(errBeforeNotInList)
	}
	l.link(item)

	if before != nil {
		// prev   before
		//      ^
		//     item
		l.updateCursors(before.Prev, item, before, item)

		if before.Prev == nil {
			// insert to the beginning of list
			l.head = item
			before.Prev = item
			item.Next = before
			// upstream passes undefined for nextOld and nextNew, which no
			// cursor matches; nil for both leaves next unchanged as well
			l.updateCursors(nil, item, nil, nil)
		} else {
			// insert between two items
			before.Prev.Next = item
			item.Prev = before.Prev
			before.Prev = item
			item.Next = before
		}
	} else {
		// tail
		//      ^
		//      item
		l.updateCursors(l.tail, item, nil, item)

		// insert to the ending of the list
		if l.tail != nil {
			// last item -> new item
			l.tail.Next = item
			// last item <- new item
			item.Prev = l.tail
		} else {
			// if list has no tail, then it also has no head
			// in this case head points to new item
			l.head = item
		}

		// tail always points to new item
		l.tail = item
	}

	return l
}

// InsertData links a new item holding data before the item before, or
// after the tail when before is nil, and returns l. It panics if before
// does not belong to l.
func (l *List[T]) InsertData(data T, before *ListItem[T]) *List[T] {
	return l.Insert(NewListItem(data), before)
}

// Remove unlinks item from l and returns it. It panics if item does not
// belong to l.
func (l *List[T]) Remove(item *ListItem[T]) *ListItem[T] {
	if item.list != l {
		panic(errItemNotInList)
	}

	//      item
	//       ^
	// prev     next
	l.updateCursors(item, item.Prev, item, item.Next)

	if item.Prev != nil {
		item.Prev.Next = item.Next
	} else {
		l.head = item.Next
	}

	if item.Next != nil {
		item.Next.Prev = item.Prev
	} else {
		l.tail = item.Prev
	}

	item.Prev = nil
	item.Next = nil
	item.list = nil
	l.len--

	return item
}

// PrependList moves the items of list before the head of l, leaving list
// empty, and returns l.
func (l *List[T]) PrependList(list *List[T]) *List[T] {
	return l.InsertList(list, l.head)
}

// AppendList moves the items of list after the tail of l, leaving list
// empty, and returns l.
func (l *List[T]) AppendList(list *List[T]) *List[T] {
	return l.InsertList(list, nil)
}

// InsertList moves the items of list before the item before, or after the
// tail when before is nil, leaving list empty, and returns l. It panics if
// list is l or before does not belong to l.
func (l *List[T]) InsertList(list *List[T], before *ListItem[T]) *List[T] {
	if list == l {
		panic(errListIntoItself)
	}
	if before != nil && before.list != l {
		panic(errBeforeNotInList)
	}

	// ignore empty lists
	if list.head == nil {
		return l
	}

	for item := list.head; item != nil; item = item.Next {
		item.list = l
	}
	l.len += list.len

	if before != nil {
		l.updateCursors(before.Prev, list.tail, before, list.head)

		// insert in the middle of dist list
		if before.Prev != nil {
			// before.prev <-> list.head
			before.Prev.Next = list.head
			list.head.Prev = before.Prev
		} else {
			l.head = list.head
		}

		before.Prev = list.tail
		list.tail.Next = before
	} else {
		l.updateCursors(l.tail, list.tail, nil, list.head)

		// insert to end of the list
		if l.tail != nil {
			// if destination list has a tail, then it also has a head,
			// but head doesn't change
			// dest tail -> source head
			l.tail.Next = list.head
			// dest tail <- source head
			list.head.Prev = l.tail
		} else {
			// if list has no a tail, then it also has no a head
			// in this case points head to new item
			l.head = list.head
		}

		// tail always start point to new item
		l.tail = list.tail
	}

	list.head = nil
	list.tail = nil
	list.len = 0
	return l
}

// Replace links newItem in place of oldItem and unlinks oldItem. It panics
// if oldItem does not belong to l or newItem is already in a list.
// Upstream replace takes an item or a list; ReplaceList takes the list.
func (l *List[T]) Replace(oldItem, newItem *ListItem[T]) {
	l.Insert(newItem, oldItem)

	l.Remove(oldItem)
}

// ReplaceList moves the items of list in place of oldItem, leaving list
// empty, and unlinks oldItem. It panics like InsertList.
func (l *List[T]) ReplaceList(oldItem *ListItem[T], list *List[T]) {
	l.InsertList(list, oldItem)

	l.Remove(oldItem)
}
