package planner

import (
	"bytes"
	"container/heap"
	"fmt"

	"github.com/genjidb/genji/database"
	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/sql/query/expr"
	"github.com/genjidb/genji/sql/scanner"
)

type sortNode struct {
	node

	sortField expr.Path
	direction scanner.Token
}

var _ operationNode = (*sortNode)(nil)

// NewSortNode creates a node that sorts a stream according to a given
// document path and a sort direction.
func NewSortNode(n Node, sortField expr.Path, direction scanner.Token) Node {
	if direction == 0 {
		direction = scanner.ASC
	}

	return &sortNode{
		node: node{
			op:   Sort,
			left: n,
		},
		sortField: sortField,
		direction: direction,
	}
}

func (n *sortNode) Bind(tx *database.Transaction, params []expr.Param) (err error) {
	return
}

func (n *sortNode) toStream(st document.Stream) (document.Stream, error) {
	return document.NewStream(&sortIterator{
		st:        st,
		sortField: n.sortField,
		direction: n.direction,
	}), nil
}

func (n *sortNode) String() string {
	dir := "ASC"
	if n.direction == scanner.DESC {
		dir = "DESC"
	}

	return fmt.Sprintf("Sort(%s %s)", n.sortField, dir)
}

type sortIterator struct {
	st        document.Stream
	sortField expr.Path
	direction scanner.Token
}

func (it *sortIterator) Iterate(fn func(d document.Document) error) error {
	h, err := it.sortStream(it.st)
	if err != nil {
		return err
	}

	for h.Len() > 0 {
		node := heap.Pop(h).(heapNode)
		err := fn(&(node.data))
		if err != nil {
			return err
		}
	}

	return nil
}

// sortStream operates a partial sort on the iterator using a heap.
// This ensures a O(k+n log n) time complexity, where k is the sum of
// OFFSET + LIMIT clauses, if provided, otherwise k = n.
// If the sorting is in ascending order, a min-heap will be used
// otherwise a max-heap will be used instead.
// Once the heap is filled entirely with the content of the table a stream is returned.
// During iteration, the stream will pop the k-smallest or k-largest elements, depending on
// the chosen sorting order (ASC or DESC).
// This function is not memory efficient as it's loading the entire stream in memory before
// returning the k-smallest or k-largest elements.
func (it *sortIterator) sortStream(st document.Stream) (heap.Interface, error) {
	path := document.Path(it.sortField)

	var h heap.Interface
	if it.direction == scanner.ASC {
		h = new(minHeap)
	} else {
		h = new(maxHeap)
	}

	heap.Init(h)

	return h, st.Iterate(func(d document.Document) error {
		// It is possible to sort by any projected field
		// or field of the original document.
		v, err := path.GetValueFromDocument(d)
		if err != nil && err != document.ErrFieldNotFound {
			return err
		}

		// If a field is not found in the projected fields
		// Look for fields in the original document.
		if err == document.ErrFieldNotFound {
			if dm, ok := d.(*documentMask); ok {
				v, err = path.GetValueFromDocument(dm.d)
				if err != nil && err != document.ErrFieldNotFound {
					return err
				}
				if err == document.ErrFieldNotFound {
					v = document.NewNullValue()
				}
			} else {
				v = document.NewNullValue()
			}
		}

		// We need to make sure sort behaviour
		// if the same with or without indexes.
		// To achieve that, the value must be encoded using the same method
		// as what the index package would do.
		var buf bytes.Buffer

		err = document.NewValueEncoder(&buf).Encode(v)
		if err != nil {
			return err
		}

		node := heapNode{
			value: buf.Bytes(),
		}
		err = node.data.Copy(d)
		if err != nil {
			return err
		}

		heap.Push(h, node)

		return nil
	})
}

type heapNode struct {
	value []byte
	data  document.FieldBuffer
}

type minHeap []heapNode

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return bytes.Compare(h[i].value, h[j].value) < 0 }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *minHeap) Push(x interface{}) {
	*h = append(*h, x.(heapNode))
}

func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type maxHeap struct {
	minHeap
}

func (h maxHeap) Less(i, j int) bool {
	return bytes.Compare(h.minHeap[i].value, h.minHeap[j].value) > 0
}
