// ahocorasick.go: implementation of the Aho-Corasick string matching
// algorithm. Actually implemented as matching against []byte rather
// than the Go string type. Throughout this code []byte is referred to
// as a blice.
//
// http://en.wikipedia.org/wiki/Aho%E2%80%93Corasick_string_matching_algorithm
//
// Copyright (c) 2013 CloudFlare, Inc.

package ahocorasick

import (
	"container/list"
	"sync"
	"sync/atomic"
)

// A node in the trie structure used to implement Aho-Corasick.
type node struct {
	child  [256]*node // trie children, used only during construction
	next   [256]*node // goto table: next[c] = node to transition to on byte c (nil = no match from root)
	suffix *node
	fail   *node
	b      []byte
	counter uint64
	index   int
	output  bool
	root    bool
}

// seenBuf wraps the deduplication slice for pooling, avoiding the need to
// take the address of a local variable (which would cause it to escape to heap).
type seenBuf struct {
	s []uint64
}

// Matcher is returned by NewMatcher and contains a list of blices to
// match against.
type Matcher struct {
	heap    sync.Pool
	root    *node
	trie    []node
	counter uint64
	extent  int
	dictLen int // number of patterns in the dictionary; bounds f.index
}

// findBlice looks for a blice in the trie starting from the root and
// returns a pointer to the node representing the end of the blice. If
// the blice is not found it returns nil.
func (m *Matcher) findBlice(b []byte) *node {
	n := &m.trie[0]

	for n != nil && len(b) > 0 {
		n = n.child[int(b[0])]
		b = b[1:]
	}

	return n
}

// getFreeNode: gets a free node structure from the Matcher's trie
// pool and updates the extent to point to the next free node.
func (m *Matcher) getFreeNode() *node {
	m.extent++

	if m.extent == 1 {
		m.root = &m.trie[0]
		m.root.root = true
	}

	return &m.trie[m.extent-1]
}

// buildTrie builds the fundamental trie structure from a set of
// blices.
func (m *Matcher) buildTrie(dictionary [][]byte) {

	// Work out the maximum size for the trie (all dictionary entries
	// are distinct plus the root). This is used to preallocate memory
	// for it.

	m.dictLen = len(dictionary)

	size := 1
	for _, blice := range dictionary {
		size += len(blice)
	}
	m.trie = make([]node, size)

	// Calling this an ignoring its argument simply allocated
	// m.trie[0] which will be the root element

	m.getFreeNode()

	// This loop builds the nodes in the trie by following through
	// each dictionary entry building the children pointers.

	for i, blice := range dictionary {
		n := m.root
		var path []byte
		for _, b := range blice {
			path = append(path, b)

			c := n.child[int(b)]

			if c == nil {
				c = m.getFreeNode()
				n.child[int(b)] = c
				c.b = make([]byte, len(path))
				copy(c.b, path)

				// Nodes directly under the root node will have the
				// root as their fail point as there are no suffixes
				// possible.

				if len(path) == 1 {
					c.fail = m.root
				}

				c.suffix = m.root
			}

			n = c
		}

		// The last value of n points to the node representing a
		// dictionary entry

		n.output = true
		n.index = i
	}

	l := new(list.List)
	l.PushBack(m.root)

	for l.Len() > 0 {
		n, _ := l.Remove(l.Front()).(*node)

		for i := 0; i < 256; i++ {
			c := n.child[i]
			if c != nil {
				l.PushBack(c)

				for j := 1; j < len(c.b); j++ {
					c.fail = m.findBlice(c.b[j:])
					if c.fail != nil {
						break
					}
				}

				if c.fail == nil {
					c.fail = m.root
				}

				for j := 1; j < len(c.b); j++ {
					s := m.findBlice(c.b[j:])
					if s != nil && s.output {
						c.suffix = s
						break
					}
				}
			}
		}
	}

	// Build the goto table: next[c] is the node to land on directly,
	// collapsing the two-step fails[c] + child[c] into one lookup.
	// All next entries must be computed before clearing any child arrays,
	// because fail links point to lower-indexed nodes whose child arrays
	// would otherwise already be cleared when we need them.
	for i := 0; i < m.extent; i++ {
		for c := 0; c < 256; c++ {
			n := &m.trie[i]
			for n.child[c] == nil && !n.root {
				n = n.fail
			}
			m.trie[i].next[c] = n.child[c]
		}
	}
	for i := 0; i < m.extent; i++ {
		m.trie[i].child = [256]*node{}
	}

	m.trie = m.trie[:m.extent]
}

// NewMatcher creates a new Matcher used to match against a set of
// blices.
func NewMatcher(dictionary [][]byte) *Matcher {
	m := new(Matcher)

	m.buildTrie(dictionary)

	return m
}

// NewStringMatcher creates a new Matcher used to match against a set
// of strings (this is a helper to make initialization easy).
func NewStringMatcher(dictionary []string) *Matcher {
	m := new(Matcher)

	var d [][]byte
	for _, s := range dictionary {
		d = append(d, []byte(s))
	}

	m.buildTrie(d)

	return m
}

// MatchInto searches in for blices and appends the indexes of matched dictionary
// entries into dst, returning the number of matches written. The caller may
// reuse dst across calls to avoid allocations.
//
// This is not a thread-safe method; see MatchThreadSafeInto instead.
func (m *Matcher) MatchInto(in []byte, dst []int) int {
	m.counter++
	before := len(dst)
	dst = match(in, m.root, m.root, func(f *node) bool {
		if f.counter != m.counter {
			f.counter = m.counter
			return true
		}
		return false
	}, dst)
	return len(dst) - before
}

// Match searches in for blices and returns all the blices found as indexes into
// the original dictionary.
//
// This is not thread-safe method, seek for MatchThreadSafe() instead.
func (m *Matcher) Match(in []byte) []int {
	m.counter++
	return match(in, m.root, m.root, func(f *node) bool {
		if f.counter != m.counter {
			f.counter = m.counter
			return true
		}
		return false
	}, nil)
}

// match is a core of matching logic. Accepts input byte slice, starting node
// and a func to check whether should we include result into response or not.
// Results are appended to dst; the updated slice is returned.
func match(in []byte, n *node, root *node, unique func(f *node) bool, dst []int) []int {
	for _, b := range in {
		// next[c] is the pre-computed goto table: the node to transition to
		// directly on byte c, following fail links as needed. nil means the
		// root has no child for c, so reset to root and skip output checks.
		f := n.next[int(b)]
		if f == nil {
			n = root
			continue
		}
		n = f

		if f.output {
			if unique(f) {
				dst = append(dst, f.index)
			}
		}

		for !f.suffix.root {
			f = f.suffix
			if unique(f) {
				dst = append(dst, f.index)
			} else {
				// There's no point working our way up the
				// suffixes if it's been done before for this call
				// to Match. The matches are already in hits.
				break
			}
		}
	}

	return dst
}

// MatchThreadSafeInto searches in for blices and appends the indexes of matched
// dictionary entries into dst, returning the number of matches written. The
// caller may reuse dst across calls to avoid allocations.
func (m *Matcher) MatchThreadSafeInto(in []byte, dst []int) int {
	generation := atomic.AddUint64(&m.counter, 1)
	n := m.root

	// Use a pooled *seenBuf indexed by dictionary index for O(1) deduplication
	// without map overhead. Dictionary indexes are dense 0..dictLen-1.
	// Pooling a *seenBuf (not *[]uint64) avoids taking the address of a local,
	// which would cause the slice header to escape to heap on every call.
	var buf *seenBuf
	if item := m.heap.Get(); item != nil {
		buf, _ = item.(*seenBuf)
	}
	if buf == nil {
		buf = &seenBuf{s: make([]uint64, m.dictLen)}
	}
	seen := buf.s

	before := len(dst)
	dst = match(in, n, m.root, func(f *node) bool {
		if seen[f.index] != generation {
			seen[f.index] = generation
			return true
		}
		return false
	}, dst)

	m.heap.Put(buf)
	return len(dst) - before
}

// MatchThreadSafe provides the same result as Match() but does it in a
// thread-safe manner. Uses a sync.Pool of seen-arrays to track the uniqueness
// of the result items.
func (m *Matcher) MatchThreadSafe(in []byte) []int {
	generation := atomic.AddUint64(&m.counter, 1)
	n := m.root

	var buf *seenBuf
	if item := m.heap.Get(); item != nil {
		buf, _ = item.(*seenBuf)
	}
	if buf == nil {
		buf = &seenBuf{s: make([]uint64, m.dictLen)}
	}
	seen := buf.s

	hits := match(in, n, m.root, func(f *node) bool {
		if seen[f.index] != generation {
			seen[f.index] = generation
			return true
		}
		return false
	}, nil)

	m.heap.Put(buf)
	return hits
}

// Contains returns true if any string matches. This can be faster
// than Match() when you do not need to know which words matched.
func (m *Matcher) Contains(in []byte) bool {
	n := m.root
	for _, b := range in {
		f := n.next[int(b)]
		if f == nil {
			n = m.root
			continue
		}
		n = f
		if f.output || !f.suffix.root {
			return true
		}
	}
	return false
}
