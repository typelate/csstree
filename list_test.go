// Ported from lib/__tests/list.js.

package csstree

import (
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"testing"
)

// Upstream uses two distinct empty objects as foo and bar and compares them
// by identity. Distinct strings give the same guarantees for a List[string]
// and let the modification tests mix them with the "ins<N>" strings they add.
const (
	foo = "foo"
	bar = "bar"
)

func getData(item *ListItem[string]) string {
	return item.Data
}

// iteratorState drops upstream's thisArg (Go callbacks are closures) and
// list (the callbacks are methods of the list they iterate).
type iteratorState struct {
	data string
	item *ListItem[string]
}

func getIteratorState(data string, item *ListItem[string]) iteratorState {
	return iteratorState{
		data: data,
		item: item,
	}
}

type iteratorStateCollector struct {
	states []iteratorState
}

func (c *iteratorStateCollector) collect(data string, item *ListItem[string]) {
	c.states = append(c.states, getIteratorState(data, item))
}

// newList returns a list of data (upstream new List().fromArray(data)).
func newList(data ...string) *List[string] {
	list := new(List[string])
	for _, d := range data {
		list.AppendData(d)
	}
	return list
}

// toArray checks every link of list before returning its data.
func toArray(t *testing.T, list *List[string]) []string {
	t.Helper()

	cursor := list.Head()
	var prev *ListItem[string]

	for cursor != nil {
		if cursor.Prev != prev {
			t.Fatal("Wrong prev reference")
		}

		if prev != nil && prev.Next != cursor {
			t.Fatal("Wrong next reference")
		}

		prev = cursor
		cursor = cursor.Next
	}

	if list.Tail() != prev {
		t.Fatal("Wrong tail reference")
	}

	if prev != nil && prev.Next != nil {
		t.Fatal("Wrong next reference")
	}

	if got := list.Len(); got != count(list) {
		t.Fatalf("Len() = %d, want %d", got, count(list))
	}

	return slices.Collect(list.All())
}

// count counts the items of list by following the links.
func count(list *List[string]) int {
	n := 0
	for item := list.Head(); item != nil; item = item.Next {
		n++
	}
	return n
}

// iterateFunc adapts one List iteration method to a common shape so the
// shared iterator tests can drive it, like upstream's list[method].bind(...).
type iterateFunc func(list *List[string], fn func(data string, item *ListItem[string]))

var iterateMethods = map[string]iterateFunc{
	"forEach": func(list *List[string], fn func(string, *ListItem[string])) {
		list.ForEach(fn)
	},
	"forEachRight": func(list *List[string], fn func(string, *ListItem[string])) {
		list.ForEachRight(fn)
	},
}

func createIteratorTests(t *testing.T, list *List[string], method string, items []*ListItem[string]) {
	iterate := func(fn func(string, *ListItem[string])) {
		iterateMethods[method](list, fn)
	}

	t.Run("iterate", func(t *testing.T) {
		collector := new(iteratorStateCollector)

		iterate(collector.collect)
		var want []iteratorState
		for _, item := range items {
			want = append(want, getIteratorState(item.Data, item))
		}
		if !slices.Equal(collector.states, want) {
			t.Errorf("%s visited %+v, want %+v", method, collector.states, want)
		}
	})

	t.Run("iterate with thisArg", func(t *testing.T) {
		t.Skip("not ported: Go callbacks are closures, there is no thisArg to bind")
	})

	t.Run("nested iterate", func(t *testing.T) {
		collector := new(iteratorStateCollector)

		iterate(func(string, *ListItem[string]) {
			iterate(collector.collect)
		})

		var want []iteratorState
		for range items {
			for _, item := range items {
				want = append(want, getIteratorState(item.Data, item))
			}
		}
		if !slices.Equal(collector.states, want) {
			t.Errorf("nested %s visited %+v, want %+v", method, collector.states, want)
		}
	})
}

var insertedData = regexp.MustCompile(`^ins\d+$`)

func createIteratorWithModificationTests(t *testing.T, list *List[string], method string, items []*ListItem[string]) {
	iterate := func(list *List[string], fn func(string, *ListItem[string])) {
		iterateMethods[method](list, fn)
	}

	t.Run("remove items on iterate", func(t *testing.T) {
		iterateList := list.Copy()
		var removed []string

		iterate(iterateList, func(data string, item *ListItem[string]) {
			removed = append(removed, data)
			iterateList.Remove(item)
		})

		if iterateList.Head() != nil {
			t.Errorf("Head() = %+v after removing every item, want nil", iterateList.Head())
		}
		var want []string
		for _, item := range items {
			want = append(want, getData(item))
		}
		if !slices.Equal(removed, want) {
			t.Errorf("removed %q, want %q", removed, want)
		}
	})

	t.Run("insert items on iterate", func(t *testing.T) {
		iterateList := list.Copy()
		var inserted []string
		var order []string
		idx := 0

		iterate(iterateList, func(data string, item *ListItem[string]) {
			if !insertedData.MatchString(data) {
				before := NewListItem("ins" + strconv.Itoa(idx))
				after := NewListItem("ins" + strconv.Itoa(idx))

				iterateList.Insert(before, item)
				iterateList.Insert(after, item.Next)
				order = append(order, data)
				inserted = append(inserted, data, "ins"+strconv.Itoa(idx))
				idx++
			}
		})

		var wantInserted []string
		for idx, item := range items {
			wantInserted = append(wantInserted, item.Data, "ins"+strconv.Itoa(idx))
		}
		if !slices.Equal(inserted, wantInserted) {
			t.Errorf("inserted %q, want %q", inserted, wantInserted)
		}

		var want []string
		for data := range list.All() {
			idx := slices.Index(order, data)
			want = append(want, "ins"+strconv.Itoa(idx), data, "ins"+strconv.Itoa(idx))
		}
		if got := slices.Collect(iterateList.All()); !slices.Equal(got, want) {
			t.Errorf("list after insertions = %q, want %q", got, want)
		}
	})
}

// assertItem replaces upstream's deepStrictEqual(item, {prev, next, data}).
func assertItem(t *testing.T, name string, item, prev, next *ListItem[string], data string) {
	t.Helper()
	if item.Prev != prev || item.Next != next || item.Data != data {
		t.Errorf("%s = {Prev: %p, Next: %p, Data: %q}, want {Prev: %p, Next: %p, Data: %q}",
			name, item.Prev, item.Next, item.Data, prev, next, data)
	}
}

// assertPanicsWith replaces upstream's assert.throws for misuse that throws.
func assertPanicsWith(t *testing.T, want error, fn func()) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		err, ok := r.(error)
		if !ok || !errors.Is(err, want) {
			t.Errorf("recovered %v, want panic with %v", r, want)
		}
	}()
	fn()
}

func TestList(t *testing.T) {
	var (
		empty *List[string]
		list1 *List[string]
		list2 *List[string]
	)

	// run is upstream's it() with its beforeEach.
	run := func(t *testing.T, name string, fn func(t *testing.T)) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			empty = new(List[string])
			list1 = newList(foo)
			list2 = newList(foo, bar)
			fn(t)
		})
	}

	run(t, ".createItem()", func(t *testing.T) {
		if got, want := *NewListItem(foo), (ListItem[string]{Prev: nil, Next: nil, Data: foo}); got != want {
			t.Errorf("NewListItem(%q) = %+v, want %+v", foo, got, want)
		}
	})

	run(t, "#size (Len)", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			list *List[string]
			want int
		}{
			{"empty", empty, 0},
			{"list1", list1, 1},
			{"list2", list2, 2},
		} {
			if got := tt.list.Len(); got != tt.want {
				t.Errorf("%s.Len() = %d, want %d", tt.name, got, tt.want)
			}
		}
	})

	run(t, "#toJSON()", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			list *List[string]
			want string
		}{
			{"empty", empty, `[]`},
			{"list1", list1, `["foo"]`},
			{"list2", list2, `["foo","bar"]`},
		} {
			got, err := json.Marshal(tt.list)
			if err != nil {
				t.Fatalf("json.Marshal(%s) error: %v", tt.name, err)
			}
			if string(got) != tt.want {
				t.Errorf("json.Marshal(%s) = %s, want %s", tt.name, got, tt.want)
			}
		}
	})

	run(t, "#isEmpty", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			list *List[string]
			want bool
		}{
			{"empty", empty, true},
			{"list1", list1, false},
			{"list2", list2, false},
		} {
			if got := tt.list.IsEmpty(); got != tt.want {
				t.Errorf("%s.IsEmpty() = %t, want %t", tt.name, got, tt.want)
			}
		}
	})

	run(t, "#first", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			list *List[string]
			want string
		}{
			{"empty", empty, ""}, // upstream: null
			{"list1", list1, foo},
			{"list2", list2, foo},
		} {
			if got := tt.list.First(); got != tt.want {
				t.Errorf("%s.First() = %q, want %q", tt.name, got, tt.want)
			}
		}
	})

	run(t, "#last", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			list *List[string]
			want string
		}{
			{"empty", empty, ""}, // upstream: null
			{"list1", list1, foo},
			{"list2", list2, bar},
		} {
			if got := tt.list.Last(); got != tt.want {
				t.Errorf("%s.Last() = %q, want %q", tt.name, got, tt.want)
			}
		}
	})

	t.Run("#forEach()", func(t *testing.T) {
		iterateList := newList(foo, bar)
		createIteratorTests(t, iterateList, "forEach", []*ListItem[string]{iterateList.Head(), iterateList.Tail()})
		createIteratorWithModificationTests(t, iterateList, "forEach", []*ListItem[string]{iterateList.Head(), iterateList.Tail()})
	})

	t.Run("#forEachRight()", func(t *testing.T) {
		iterateList := newList(foo, bar)
		createIteratorTests(t, iterateList, "forEachRight", []*ListItem[string]{iterateList.Tail(), iterateList.Head()})
		createIteratorWithModificationTests(t, iterateList, "forEachRight", []*ListItem[string]{iterateList.Tail(), iterateList.Head()})
	})

	run(t, "All and Backward", func(t *testing.T) {
		if got, want := slices.Collect(list2.All()), []string{foo, bar}; !slices.Equal(got, want) {
			t.Errorf("All() = %q, want %q", got, want)
		}
		if got, want := slices.Collect(list2.Backward()), []string{bar, foo}; !slices.Equal(got, want) {
			t.Errorf("Backward() = %q, want %q", got, want)
		}
		if got := slices.Collect(empty.Backward()); len(got) != 0 {
			t.Errorf("empty.Backward() = %q, want none", got)
		}
		for data := range list2.Backward() {
			if data != bar {
				t.Errorf("Backward() after break yielded %q", data)
			}
			break
		}
	})

	t.Run("#clear()", func(t *testing.T) {
		run(t, "empty list", func(t *testing.T) {
			if empty.Head() != nil || empty.Tail() != nil {
				t.Fatalf("empty list has Head() = %p, Tail() = %p, want nil", empty.Head(), empty.Tail())
			}
			empty.Clear()
			if empty.Head() != nil || empty.Tail() != nil {
				t.Errorf("after Clear() Head() = %p, Tail() = %p, want nil", empty.Head(), empty.Tail())
			}
		})

		run(t, "non-empty list", func(t *testing.T) {
			if list2.Head() == nil || list2.Tail() == nil {
				t.Fatalf("list2 has Head() = %p, Tail() = %p, want non-nil", list2.Head(), list2.Tail())
			}
			list2.Clear()
			if list2.Head() != nil || list2.Tail() != nil {
				t.Errorf("after Clear() Head() = %p, Tail() = %p, want nil", list2.Head(), list2.Tail())
			}
		})
	})

	run(t, "#copy()", func(t *testing.T) {
		copied := list2.Copy()

		if copied == list2 {
			t.Error("Copy() returned the same list")
		}
		if copied.Head() == list2.Head() {
			t.Error("Copy().Head() is the same item as Head()")
		}
		if copied.Head().Data != list2.Head().Data {
			t.Errorf("Copy().Head().Data = %q, want %q", copied.Head().Data, list2.Head().Data)
		}
		if copied.Tail() == list2.Tail() {
			t.Error("Copy().Tail() is the same item as Tail()")
		}
		if copied.Tail().Data != list2.Tail().Data {
			t.Errorf("Copy().Tail().Data = %q, want %q", copied.Tail().Data, list2.Tail().Data)
		}
	})

	run(t, "#prepend()", func(t *testing.T) {
		const qux = "qux"

		if got, want := toArray(t, empty.Prepend(NewListItem(qux))), []string{qux}; !slices.Equal(got, want) {
			t.Errorf("empty.Prepend = %q, want %q", got, want)
		}
		if got, want := toArray(t, list1.Prepend(NewListItem(qux))), []string{qux, foo}; !slices.Equal(got, want) {
			t.Errorf("list1.Prepend = %q, want %q", got, want)
		}
		if got, want := toArray(t, list2.Prepend(NewListItem(qux))), []string{qux, foo, bar}; !slices.Equal(got, want) {
			t.Errorf("list2.Prepend = %q, want %q", got, want)
		}
		if got := list2.Prepend(NewListItem("")); got != list2 {
			t.Error("Prepend did not return its receiver")
		}
	})

	run(t, "#prependData()", func(t *testing.T) {
		const qux = "qux"

		if got, want := toArray(t, empty.PrependData(qux)), []string{qux}; !slices.Equal(got, want) {
			t.Errorf("empty.PrependData = %q, want %q", got, want)
		}
		if got, want := toArray(t, list1.PrependData(qux)), []string{qux, foo}; !slices.Equal(got, want) {
			t.Errorf("list1.PrependData = %q, want %q", got, want)
		}
		if got, want := toArray(t, list2.PrependData(qux)), []string{qux, foo, bar}; !slices.Equal(got, want) {
			t.Errorf("list2.PrependData = %q, want %q", got, want)
		}
		if got := list2.PrependData(qux); got != list2 {
			t.Error("PrependData did not return its receiver")
		}
	})

	run(t, "#append()", func(t *testing.T) {
		const qux = "qux"

		if got, want := toArray(t, empty.Append(NewListItem(qux))), []string{qux}; !slices.Equal(got, want) {
			t.Errorf("empty.Append = %q, want %q", got, want)
		}
		if got, want := toArray(t, list1.Append(NewListItem(qux))), []string{foo, qux}; !slices.Equal(got, want) {
			t.Errorf("list1.Append = %q, want %q", got, want)
		}
		if got, want := toArray(t, list2.Append(NewListItem(qux))), []string{foo, bar, qux}; !slices.Equal(got, want) {
			t.Errorf("list2.Append = %q, want %q", got, want)
		}
		if got := list2.Append(NewListItem("")); got != list2 {
			t.Error("Append did not return its receiver")
		}
	})

	run(t, "#appendData()", func(t *testing.T) {
		const qux = "qux"

		if got, want := toArray(t, empty.AppendData(qux)), []string{qux}; !slices.Equal(got, want) {
			t.Errorf("empty.AppendData = %q, want %q", got, want)
		}
		if got, want := toArray(t, list1.AppendData(qux)), []string{foo, qux}; !slices.Equal(got, want) {
			t.Errorf("list1.AppendData = %q, want %q", got, want)
		}
		if got, want := toArray(t, list2.AppendData(qux)), []string{foo, bar, qux}; !slices.Equal(got, want) {
			t.Errorf("list2.AppendData = %q, want %q", got, want)
		}
		if got := list2.AppendData(qux); got != list2 {
			t.Error("AppendData did not return its receiver")
		}
	})

	t.Run("#insert()", func(t *testing.T) {
		run(t, "should append when no ref item", func(t *testing.T) {
			const afterTail = "afterTail"
			res := list2.Insert(NewListItem(afterTail), nil)

			if res != list2 {
				t.Error("Insert did not return its receiver")
			}
			if got := list2.Tail().Data; got != afterTail {
				t.Errorf("Tail().Data = %q, want %q", got, afterTail)
			}
		})

		run(t, "should insert before ref item", func(t *testing.T) {
			const beforeHead = "beforeHead"
			res := list2.Insert(NewListItem(beforeHead), list2.Head())

			if res != list2 {
				t.Error("Insert did not return its receiver")
			}
			if got := list2.Head().Data; got != beforeHead {
				t.Errorf("Head().Data = %q, want %q", got, beforeHead)
			}
		})

		run(t, "insert in the middle", func(t *testing.T) {
			const qux = "qux"
			inserted := NewListItem(qux)
			list2head := list2.Head()
			list2tail := list2.Tail()
			res := list2.Insert(inserted, list2.Tail())

			if res != list2 {
				t.Error("Insert did not return its receiver")
			}
			assertItem(t, "list2head", list2head, nil, inserted, foo)
			assertItem(t, "inserted", inserted, list2head, list2tail, qux)
			assertItem(t, "list2tail", list2tail, inserted, nil, bar)
		})

		run(t, "insert the item before an item that doesn't belong to list", func(t *testing.T) {
			inserted := NewListItem("")

			assertPanicsWith(t, errBeforeNotInList, func() {
				list2.Insert(inserted, list1.Head())
			})
		})
	})

	t.Run("#insertData()", func(t *testing.T) {
		run(t, "should append when no ref item", func(t *testing.T) {
			const afterTail = "afterTail"
			res := list2.InsertData(afterTail, nil)

			if res != list2 {
				t.Error("InsertData did not return its receiver")
			}
			if got := list2.Tail().Data; got != afterTail {
				t.Errorf("Tail().Data = %q, want %q", got, afterTail)
			}
		})

		run(t, "should insert before ref item", func(t *testing.T) {
			const beforeHead = "beforeHead"
			res := list2.InsertData(beforeHead, list2.Head())

			if res != list2 {
				t.Error("InsertData did not return its receiver")
			}
			if got := list2.Head().Data; got != beforeHead {
				t.Errorf("Head().Data = %q, want %q", got, beforeHead)
			}
		})

		run(t, "insert in the middle", func(t *testing.T) {
			const qux = "qux"
			list2head := list2.Head()
			list2tail := list2.Tail()
			res := list2.InsertData(qux, list2.Tail())

			if res != list2 {
				t.Error("InsertData did not return its receiver")
			}
			assertItem(t, "list2head", list2head, nil, list2tail.Prev, foo)
			assertItem(t, "list2tail.Prev", list2tail.Prev, list2head, list2tail, qux)
			assertItem(t, "list2tail", list2tail, list2tail.Prev, nil, bar)
		})

		run(t, "insert the item before an item that doesn't belong to list", func(t *testing.T) {
			assertPanicsWith(t, errBeforeNotInList, func() {
				list2.InsertData("", list1.Head())
			})
		})
	})

	t.Run("#remove()", func(t *testing.T) {
		run(t, "clear a list", func(t *testing.T) {
			head := list2.Head()
			tail := list2.Tail()

			list2.Remove(list2.Tail())
			list2.Remove(list2.Head())

			if list2.Head() != nil || list2.Tail() != nil {
				t.Errorf("Head() = %p, Tail() = %p, want nil", list2.Head(), list2.Tail())
			}
			assertItem(t, "head", head, nil, nil, foo)
			assertItem(t, "tail", tail, nil, nil, bar)
		})

		run(t, "clear a list in reverse order", func(t *testing.T) {
			head := list2.Head()
			tail := list2.Tail()

			list2.Remove(list2.Head())
			list2.Remove(list2.Tail())

			if list2.Head() != nil || list2.Tail() != nil {
				t.Errorf("Head() = %p, Tail() = %p, want nil", list2.Head(), list2.Tail())
			}
			assertItem(t, "head", head, nil, nil, foo)
			assertItem(t, "tail", tail, nil, nil, bar)
		})

		run(t, "remove head item that doesn't belong to list", func(t *testing.T) {
			assertPanicsWith(t, errItemNotInList, func() {
				list1.Remove(list2.Head())
			})
		})

		run(t, "remove tail item that doesn't belong to list", func(t *testing.T) {
			assertPanicsWith(t, errItemNotInList, func() {
				list1.Remove(list2.Tail())
			})
		})
	})

	t.Run("#prependList()", func(t *testing.T) {
		run(t, "prepend non-empty list to non-empty list", func(t *testing.T) {
			list1head := list1.Head()
			list2head := list2.Head()
			res := list2.PrependList(list1)

			if res != list2 {
				t.Error("PrependList did not return its receiver")
			}
			if list1.Head() != nil {
				t.Errorf("list1.Head() = %p, want nil", list1.Head())
			}
			if list2.Head() != list1head {
				t.Errorf("list2.Head() = %p, want list1head %p", list2.Head(), list1head)
			}
			assertItem(t, "list1head", list1head, nil, list2head, foo)
			// upstream expects data bar here, which passes only because foo
			// and bar are deep-equal empty objects; list2head holds foo.
			assertItem(t, "list2head", list2head, list1head, list2.Tail(), foo)
		})

		run(t, "prepend non-empty list to empty list", func(t *testing.T) {
			list2head := list2.Head()
			list2tail := list2.Tail()
			res := empty.PrependList(list2)

			if res != empty {
				t.Error("PrependList did not return its receiver")
			}
			if empty.Head() != list2head || empty.Tail() != list2tail {
				t.Errorf("Head() = %p, Tail() = %p, want %p, %p", empty.Head(), empty.Tail(), list2head, list2tail)
			}
			if list2.Head() != nil || list2.Tail() != nil {
				t.Errorf("list2 Head() = %p, Tail() = %p, want nil", list2.Head(), list2.Tail())
			}
		})

		run(t, "prepend empty list to non-empty", func(t *testing.T) {
			list2head := list2.Head()
			res := list2.PrependList(empty)

			if res != list2 {
				t.Error("PrependList did not return its receiver")
			}
			if empty.Head() != nil {
				t.Errorf("empty.Head() = %p, want nil", empty.Head())
			}
			if list2.Head() != list2head {
				t.Errorf("list2.Head() = %p, want %p", list2.Head(), list2head)
			}
		})
	})

	t.Run("#appendList()", func(t *testing.T) {
		run(t, "append non-empty list to non-empty list", func(t *testing.T) {
			list1head := list1.Head()
			list2tail := list2.Tail()
			res := list2.AppendList(list1)

			if res != list2 {
				t.Error("AppendList did not return its receiver")
			}
			if list1.Head() != nil {
				t.Errorf("list1.Head() = %p, want nil", list1.Head())
			}
			if list2.Tail() != list1head {
				t.Errorf("list2.Tail() = %p, want list1head %p", list2.Tail(), list1head)
			}
			assertItem(t, "list1head", list1head, list2tail, nil, foo)
			assertItem(t, "list2tail", list2tail, list2.Head(), list1head, bar)
		})

		run(t, "append non-empty list to empty list", func(t *testing.T) {
			list2head := list2.Head()
			list2tail := list2.Tail()
			res := empty.AppendList(list2)

			if res != empty {
				t.Error("AppendList did not return its receiver")
			}
			if empty.Head() != list2head || empty.Tail() != list2tail {
				t.Errorf("Head() = %p, Tail() = %p, want %p, %p", empty.Head(), empty.Tail(), list2head, list2tail)
			}
			if list2.Head() != nil || list2.Tail() != nil {
				t.Errorf("list2 Head() = %p, Tail() = %p, want nil", list2.Head(), list2.Tail())
			}
		})

		run(t, "append empty list to non-empty", func(t *testing.T) {
			list2tail := list2.Tail()
			res := list2.AppendList(empty)

			if res != list2 {
				t.Error("AppendList did not return its receiver")
			}
			if empty.Head() != nil {
				t.Errorf("empty.Head() = %p, want nil", empty.Head())
			}
			if list2.Tail() != list2tail {
				t.Errorf("list2.Tail() = %p, want %p", list2.Tail(), list2tail)
			}
		})
	})

	t.Run("#insertList()", func(t *testing.T) {
		run(t, "add non-empty list to non-empty list", func(t *testing.T) {
			list1head := list1.Head()
			res := list2.InsertList(list1, list2.Tail())

			if res != list2 {
				t.Error("InsertList did not return its receiver")
			}
			if list1.Head() != nil || list1.Tail() != nil {
				t.Errorf("list1 Head() = %p, Tail() = %p, want nil", list1.Head(), list1.Tail())
			}
			assertItem(t, "list1head", list1head, list2.Head(), list2.Tail(), foo)
			// upstream expects data bar here, which passes only because foo
			// and bar are deep-equal empty objects; list2.head holds foo.
			assertItem(t, "list2.Head()", list2.Head(), nil, list1head, foo)
			assertItem(t, "list2.Tail()", list2.Tail(), list1head, nil, bar)
		})

		run(t, "add non-empty list to empty list", func(t *testing.T) {
			list2head := list2.Head()
			list2tail := list2.Tail()
			res := empty.InsertList(list2, nil)

			if res != empty {
				t.Error("InsertList did not return its receiver")
			}
			if empty.Head() != list2head || empty.Tail() != list2tail {
				t.Errorf("Head() = %p, Tail() = %p, want %p, %p", empty.Head(), empty.Tail(), list2head, list2tail)
			}
			if list2.Head() != nil || list2.Tail() != nil {
				t.Errorf("list2 Head() = %p, Tail() = %p, want nil", list2.Head(), list2.Tail())
			}
		})

		run(t, "add empty list to non-empty", func(t *testing.T) {
			list2tail := list2.Tail()
			res := list2.InsertList(empty, nil)

			if res != list2 {
				t.Error("InsertList did not return its receiver")
			}
			if empty.Head() != nil {
				t.Errorf("empty.Head() = %p, want nil", empty.Head())
			}
			if list2.Tail() != list2tail {
				t.Errorf("list2.Tail() = %p, want %p", list2.Tail(), list2tail)
			}
		})
	})

	t.Run("#replace()", func(t *testing.T) {
		run(t, "replace for an item", func(t *testing.T) {
			const qux = "qux"
			list2.Replace(list2.Tail(), NewListItem(qux))

			if got, want := toArray(t, list2), []string{foo, qux}; !slices.Equal(got, want) {
				t.Errorf("list2 after Replace = %q, want %q", got, want)
			}
		})

		run(t, "replace for a list", func(t *testing.T) {
			list2.ReplaceList(list2.Tail(), list1)

			if got, want := toArray(t, list2), []string{foo, foo}; !slices.Equal(got, want) {
				t.Errorf("list2 after ReplaceList = %q, want %q", got, want)
			}
		})
	})
}

// Not in upstream: upstream only mutates around the current item, which
// never needs a cursor update. Removing or inserting an item the iteration
// has not reached yet must be seen by the iteration, which only the cursor
// updates guarantee.
func TestListMutateAhead(t *testing.T) {
	type mutateFunc = func(list *List[string], data string, item *ListItem[string])

	for _, tt := range []struct {
		name    string
		reverse bool
		mutate  mutateFunc
		want    []string
	}{
		{
			name: "remove next",
			mutate: func(list *List[string], data string, item *ListItem[string]) {
				if data == "a" {
					list.Remove(item.Next)
				}
			},
			want: []string{"a", "c"},
		},
		{
			name: "insert before next",
			mutate: func(list *List[string], data string, item *ListItem[string]) {
				if data == "a" {
					list.InsertData("x", item.Next)
				}
			},
			want: []string{"a", "x", "b", "c"},
		},
		{
			name:    "remove prev",
			reverse: true,
			mutate: func(list *List[string], data string, item *ListItem[string]) {
				if data == "c" {
					list.Remove(item.Prev)
				}
			},
			want: []string{"c", "a"},
		},
		{
			name:    "insert before current",
			reverse: true,
			mutate: func(list *List[string], data string, item *ListItem[string]) {
				if data == "c" {
					list.InsertData("x", item)
				}
			},
			want: []string{"c", "x", "b", "a"},
		},
		{
			name: "clear",
			mutate: func(list *List[string], data string, _ *ListItem[string]) {
				if data == "a" {
					list.Clear()
				}
			},
			want: []string{"a"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			list := newList("a", "b", "c")
			var visited []string

			visit := func(data string, item *ListItem[string]) {
				visited = append(visited, data)
				tt.mutate(list, data, item)
			}
			if tt.reverse {
				list.ForEachRight(visit)
			} else {
				list.ForEach(visit)
			}

			if !slices.Equal(visited, tt.want) {
				t.Errorf("visited %q, want %q", visited, tt.want)
			}
			if list.cursor != nil {
				t.Error("the iteration left a cursor on the list")
			}
		})
	}
}

// Not in upstream: an item belongs to one list, and the methods taking an
// item panic rather than corrupt a list when it belongs to another one.
func TestListOwnership(t *testing.T) {
	for _, tt := range []struct {
		name string
		want error
		fn   func(a, b *List[string])
	}{
		{"Remove an item of another list", errItemNotInList, func(a, b *List[string]) { b.Remove(a.Head().Next) }},
		{"Remove an unlinked item", errItemNotInList, func(_, b *List[string]) { b.Remove(NewListItem("x")) }},
		{"Remove a removed item", errItemNotInList, func(a, _ *List[string]) {
			item := a.Head()
			a.Remove(item)
			a.Remove(item)
		}},
		{"Insert an item of another list", errItemInList, func(a, b *List[string]) { b.Insert(a.Head(), nil) }},
		{"Insert an item of the same list", errItemInList, func(a, _ *List[string]) { a.Insert(a.Head(), nil) }},
		{"Append an item of another list", errItemInList, func(a, b *List[string]) { b.Append(a.Tail()) }},
		{"Prepend an item of another list", errItemInList, func(a, b *List[string]) { b.Prepend(a.Tail()) }},
		{"Insert before an item of another list", errBeforeNotInList, func(a, b *List[string]) { b.InsertData("x", a.Tail()) }},
		{"Replace an item of another list", errBeforeNotInList, func(a, b *List[string]) { b.Replace(a.Tail(), NewListItem("x")) }},
		{"Replace with a linked item", errItemInList, func(a, b *List[string]) { b.Replace(b.Head(), a.Head()) }},
		{"InsertList before an item of another list", errBeforeNotInList, func(a, b *List[string]) { b.InsertList(newList("x"), a.Head()) }},
		{"InsertList into itself", errListIntoItself, func(a, _ *List[string]) { a.AppendList(a) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := newList("a1", "a2", "a3")
			b := newList("b1", "b2")

			assertPanicsWith(t, tt.want, func() { tt.fn(a, b) })

			if got, want := toArray(t, b), []string{"b1", "b2"}; !slices.Equal(got, want) {
				t.Errorf("b = %q after the panic, want %q", got, want)
			}
		})
	}

	t.Run("moved items belong to their new list", func(t *testing.T) {
		a := newList("a1", "a2")
		b := newList("b1")
		item := a.Head()

		b.AppendList(a)
		b.Remove(item)
		if got, want := toArray(t, b), []string{"b1", "a2"}; !slices.Equal(got, want) {
			t.Errorf("b = %q, want %q", got, want)
		}
		if got := toArray(t, a); len(got) != 0 {
			t.Errorf("a = %q, want empty", got)
		}
	})

	t.Run("a removed item can be inserted again", func(t *testing.T) {
		a := newList("a1", "a2")
		b := newList("b1")

		b.Append(a.Remove(a.Head()))
		if got, want := toArray(t, b), []string{"b1", "a1"}; !slices.Equal(got, want) {
			t.Errorf("b = %q, want %q", got, want)
		}
		if got, want := toArray(t, a), []string{"a2"}; !slices.Equal(got, want) {
			t.Errorf("a = %q, want %q", got, want)
		}
	})

	t.Run("a cleared item can be inserted again", func(t *testing.T) {
		a := newList("a1")
		item := a.Head()
		a.Clear()

		a.Append(item)
		if got, want := toArray(t, a), []string{"a1"}; !slices.Equal(got, want) {
			t.Errorf("a = %q, want %q", got, want)
		}
	})
}
