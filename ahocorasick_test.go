// ahocorasick_test.go: test suite for ahocorasick
//
// Copyright (c) 2013 CloudFlare, Inc.

package ahocorasick

import (
	bytespkg "bytes"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func assert(t *testing.T, b bool) {
	if !b {
		t.Fail()
	}
}

func TestMatchers(t *testing.T) {
	cases := []struct {
		checkIndices func(t *testing.T, n int, hits []int)
		checkPos     func(t *testing.T, n int, positions []Position)
		name         string
		patterns     []string
		input        []byte
	}{
		{
			name:     "NoPatterns",
			patterns: []string{},
			input:    []byte("foo bar baz"),
			checkIndices: func(t *testing.T, n int, _ []int) {
				t.Helper()
				assert(t, n == 0)
			},
			checkPos: func(t *testing.T, n int, _ []Position) {
				t.Helper()
				assert(t, n == 0)
			},
		},
		{
			name:     "NoData",
			patterns: []string{"foo", "baz", "bar"},
			input:    []byte(""),
			checkIndices: func(t *testing.T, n int, _ []int) {
				t.Helper()
				assert(t, n == 0)
			},
			checkPos: func(t *testing.T, n int, _ []Position) {
				t.Helper()
				assert(t, n == 0)
			},
		},
		{
			name:     "Suffixes",
			patterns: []string{"Superman", "uperman", "perman", "erman"},
			input:    []byte("The Man Of Steel: Superman"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 4)
				assert(t, hits[0] == 0)
				assert(t, hits[1] == 1)
				assert(t, hits[2] == 2)
				assert(t, hits[3] == 3)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 4)
				assert(t, positions[0] == Position{Index: 0, Start: 18, End: 26})
				assert(t, positions[1] == Position{Index: 1, Start: 19, End: 26})
				assert(t, positions[2] == Position{Index: 2, Start: 20, End: 26})
				assert(t, positions[3] == Position{Index: 3, Start: 21, End: 26})
			},
		},
		{
			name:     "Prefixes",
			patterns: []string{"Superman", "Superma", "Superm", "Super"},
			input:    []byte("The Man Of Steel: Superman"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 4)
				assert(t, hits[0] == 3)
				assert(t, hits[1] == 2)
				assert(t, hits[2] == 1)
				assert(t, hits[3] == 0)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 4)
				assert(t, positions[0] == Position{Index: 3, Start: 18, End: 23})
				assert(t, positions[1] == Position{Index: 2, Start: 18, End: 24})
				assert(t, positions[2] == Position{Index: 1, Start: 18, End: 25})
				assert(t, positions[3] == Position{Index: 0, Start: 18, End: 26})
			},
		},
		{
			name:     "Interior",
			patterns: []string{"Steel", "tee", "e"},
			input:    []byte("The Man Of Steel: Superman"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 3)
				assert(t, hits[0] == 2)
				assert(t, hits[1] == 1)
				assert(t, hits[2] == 0)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				// "e" at 2, "e" at 13, "tee" at 12-15 with "e" suffix at 14, "Steel" at 11-16, "e" at 21
				t.Helper()
				assert(t, n == 6)
				assert(t, positions[0] == Position{Index: 2, Start: 2, End: 3})
				assert(t, positions[1] == Position{Index: 2, Start: 13, End: 14})
				assert(t, positions[2] == Position{Index: 1, Start: 12, End: 15})
				assert(t, positions[3] == Position{Index: 2, Start: 14, End: 15})
				assert(t, positions[4] == Position{Index: 0, Start: 11, End: 16})
				assert(t, positions[5] == Position{Index: 2, Start: 21, End: 22})
			},
		},
		{
			name:     "AtStart",
			patterns: []string{"The", "Th", "he"},
			input:    []byte("The Man Of Steel: Superman"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 3)
				assert(t, hits[0] == 1)
				assert(t, hits[1] == 0)
				assert(t, hits[2] == 2)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 3)
				assert(t, positions[0] == Position{Index: 1, Start: 0, End: 2})
				assert(t, positions[1] == Position{Index: 0, Start: 0, End: 3})
				assert(t, positions[2] == Position{Index: 2, Start: 1, End: 3})
			},
		},
		{
			name:     "AtEnd",
			patterns: []string{"teel", "eel", "el"},
			input:    []byte("The Man Of Steel: Superman"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 3)
				assert(t, hits[0] == 0)
				assert(t, hits[1] == 1)
				assert(t, hits[2] == 2)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 3)
				assert(t, positions[0] == Position{Index: 0, Start: 12, End: 16})
				assert(t, positions[1] == Position{Index: 1, Start: 13, End: 16})
				assert(t, positions[2] == Position{Index: 2, Start: 14, End: 16})
			},
		},
		{
			name:     "Overlapping",
			patterns: []string{"Man ", "n Of", "Of S"},
			input:    []byte("The Man Of Steel"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 3)
				assert(t, hits[0] == 0)
				assert(t, hits[1] == 1)
				assert(t, hits[2] == 2)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 3)
				assert(t, positions[0] == Position{Index: 0, Start: 4, End: 8})
				assert(t, positions[1] == Position{Index: 1, Start: 6, End: 10})
				assert(t, positions[2] == Position{Index: 2, Start: 8, End: 12})
			},
		},
		{
			name:     "Multiple",
			patterns: []string{"The", "Man", "an"},
			input:    []byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 3)
				assert(t, hits[0] == 1)
				assert(t, hits[1] == 2)
				assert(t, hits[2] == 0)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 10)
				assert(t, positions[0] == Position{Index: 1, Start: 2, End: 5})
				assert(t, positions[1] == Position{Index: 2, Start: 3, End: 5})
				assert(t, positions[2] == Position{Index: 2, Start: 10, End: 12})
				assert(t, positions[3] == Position{Index: 2, Start: 16, End: 18})
				assert(t, positions[4] == Position{Index: 2, Start: 23, End: 25})
				assert(t, positions[5] == Position{Index: 1, Start: 36, End: 39})
				assert(t, positions[6] == Position{Index: 2, Start: 37, End: 39})
				assert(t, positions[7] == Position{Index: 2, Start: 42, End: 44})
				assert(t, positions[8] == Position{Index: 0, Start: 48, End: 51})
				assert(t, positions[9] == Position{Index: 2, Start: 53, End: 55})
			},
		},
		{
			name:     "SingleCharacter",
			patterns: []string{"a", "M", "z"},
			input:    []byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 2)
				assert(t, hits[0] == 1)
				assert(t, hits[1] == 0)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 13)
				assert(t, positions[0] == Position{Index: 1, Start: 2, End: 3})
				assert(t, positions[1] == Position{Index: 0, Start: 3, End: 4})
				assert(t, positions[2] == Position{Index: 0, Start: 10, End: 11})
				assert(t, positions[3] == Position{Index: 0, Start: 16, End: 17})
				assert(t, positions[4] == Position{Index: 0, Start: 18, End: 19})
				assert(t, positions[5] == Position{Index: 0, Start: 23, End: 24})
				assert(t, positions[6] == Position{Index: 0, Start: 25, End: 26})
				assert(t, positions[7] == Position{Index: 0, Start: 27, End: 28})
				assert(t, positions[8] == Position{Index: 1, Start: 36, End: 37})
				assert(t, positions[9] == Position{Index: 0, Start: 37, End: 38})
				assert(t, positions[10] == Position{Index: 0, Start: 42, End: 43})
				assert(t, positions[11] == Position{Index: 0, Start: 53, End: 54})
				assert(t, positions[12] == Position{Index: 0, Start: 55, End: 56})
			},
		},
		{
			name:     "Nothing",
			patterns: []string{"baz", "bar", "foo"},
			input:    []byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"),
			checkIndices: func(t *testing.T, n int, _ []int) {
				t.Helper()
				assert(t, n == 0)
			},
			checkPos: func(t *testing.T, n int, _ []Position) {
				t.Helper()
				assert(t, n == 0)
			},
		},
		{
			name:     "Wikipedia_Example_1",
			patterns: []string{"a", "ab", "bc", "bca", "c", "caa"},
			input:    []byte("abccab"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 4)
				assert(t, hits[0] == 0)
				assert(t, hits[1] == 1)
				assert(t, hits[2] == 2)
				assert(t, hits[3] == 4)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 7)
				assert(t, positions[0] == Position{Index: 0, Start: 0, End: 1})
				assert(t, positions[1] == Position{Index: 1, Start: 0, End: 2})
				assert(t, positions[2] == Position{Index: 2, Start: 1, End: 3})
				assert(t, positions[3] == Position{Index: 4, Start: 2, End: 3})
				assert(t, positions[4] == Position{Index: 4, Start: 3, End: 4})
				assert(t, positions[5] == Position{Index: 0, Start: 4, End: 5})
				assert(t, positions[6] == Position{Index: 1, Start: 4, End: 6})
			},
		},
		{
			name:     "Wikipedia_Example_2",
			patterns: []string{"a", "ab", "bc", "bca", "c", "caa"},
			input:    []byte("bccab"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 4)
				assert(t, hits[0] == 2)
				assert(t, hits[1] == 4)
				assert(t, hits[2] == 0)
				assert(t, hits[3] == 1)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 5)
				assert(t, positions[0] == Position{Index: 2, Start: 0, End: 2})
				assert(t, positions[1] == Position{Index: 4, Start: 1, End: 2})
				assert(t, positions[2] == Position{Index: 4, Start: 2, End: 3})
				assert(t, positions[3] == Position{Index: 0, Start: 3, End: 4})
				assert(t, positions[4] == Position{Index: 1, Start: 3, End: 5})
			},
		},
		{
			name:     "Wikipedia_Example_3",
			patterns: []string{"a", "ab", "bc", "bca", "c", "caa"},
			input:    []byte("bccb"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 2)
				assert(t, hits[0] == 2)
				assert(t, hits[1] == 4)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 3)
				assert(t, positions[0] == Position{Index: 2, Start: 0, End: 2})
				assert(t, positions[1] == Position{Index: 4, Start: 1, End: 2})
				assert(t, positions[2] == Position{Index: 4, Start: 2, End: 3})
			},
		},
		{
			name:     "UserAgent_macintosh_mac_mozilla_safari",
			patterns: []string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"},
			input:    []byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 4)
				assert(t, hits[0] == 0)
				assert(t, hits[1] == 1)
				assert(t, hits[2] == 2)
				assert(t, hits[3] == 3)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				// "Mozilla" once, "Macintosh" with "Mac" prefix, "Mac" again, "Safari" once
				t.Helper()
				assert(t, n == 5)
				assert(t, positions[0] == Position{Index: 0, Start: 0, End: 7})
				assert(t, positions[1] == Position{Index: 1, Start: 13, End: 16})
				assert(t, positions[2] == Position{Index: 2, Start: 13, End: 22})
				assert(t, positions[3] == Position{Index: 1, Start: 30, End: 33})
				assert(t, positions[4] == Position{Index: 3, Start: 107, End: 113})
			},
		},
		{
			name:     "UserAgent_mac_mozilla_safari",
			patterns: []string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"},
			input:    []byte("Mozilla/5.0 (Mac; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 3)
				assert(t, hits[0] == 0)
				assert(t, hits[1] == 1)
				assert(t, hits[2] == 3)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 4)
				assert(t, positions[0] == Position{Index: 0, Start: 0, End: 7})
				assert(t, positions[1] == Position{Index: 1, Start: 13, End: 16})
				assert(t, positions[2] == Position{Index: 1, Start: 24, End: 27})
				assert(t, positions[3] == Position{Index: 3, Start: 101, End: 107})
			},
		},
		{
			name:     "UserAgent_mozilla_safari",
			patterns: []string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"},
			input:    []byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 2)
				assert(t, hits[0] == 0)
				assert(t, hits[1] == 3)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 2)
				assert(t, positions[0] == Position{Index: 0, Start: 0, End: 7})
				assert(t, positions[1] == Position{Index: 3, Start: 106, End: 112})
			},
		},
		{
			name:     "UserAgent_mozilla_only",
			patterns: []string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"},
			input:    []byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"),
			checkIndices: func(t *testing.T, n int, hits []int) {
				t.Helper()
				assert(t, n == 1)
				assert(t, hits[0] == 0)
			},
			checkPos: func(t *testing.T, n int, positions []Position) {
				t.Helper()
				assert(t, n == 1)
				assert(t, positions[0] == Position{Index: 0, Start: 0, End: 7})
			},
		},
		{
			name:     "UserAgent_no_matches",
			patterns: []string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"},
			input:    []byte("Mazilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"),
			checkIndices: func(t *testing.T, n int, _ []int) {
				t.Helper()
				assert(t, n == 0)
			},
			checkPos: func(t *testing.T, n int, _ []Position) {
				t.Helper()
				assert(t, n == 0)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewStringMatcher(tc.patterns)

			t.Run("Match", func(t *testing.T) {
				hits := m.Match(tc.input)
				tc.checkIndices(t, len(hits), hits)
			})

			t.Run("MatchThreadSafe", func(t *testing.T) {
				t.Parallel()
				hits := m.MatchThreadSafe(tc.input)
				tc.checkIndices(t, len(hits), hits)
			})

			t.Run("MatchInto", func(t *testing.T) {
				dst := make([]int, 0, 8)
				n := m.MatchInto(tc.input, &dst)
				tc.checkIndices(t, n, dst[:n])
			})

			t.Run("MatchThreadSafeInto", func(t *testing.T) {
				t.Parallel()
				dst := make([]int, 0, 8)
				n := m.MatchThreadSafeInto(tc.input, &dst)
				tc.checkIndices(t, n, dst[:n])
			})

			t.Run("MatchPositions", func(t *testing.T) {
				positions := m.MatchPositions(tc.input)
				tc.checkPos(t, len(positions), positions)
			})

			t.Run("MatchPositionsThreadSafe", func(t *testing.T) {
				t.Parallel()
				positions := m.MatchPositionsThreadSafe(tc.input)
				tc.checkPos(t, len(positions), positions)
			})

			t.Run("MatchPositionsInto", func(t *testing.T) {
				dst := make([]Position, 0, 16)
				n := m.MatchPositionsInto(tc.input, &dst)
				tc.checkPos(t, n, dst[:n])
			})

			t.Run("MatchPositionsThreadSafeInto", func(t *testing.T) {
				t.Parallel()
				dst := make([]Position, 0, 16)
				n := m.MatchPositionsThreadSafeInto(tc.input, &dst)
				tc.checkPos(t, n, dst[:n])
			})
		})
	}
}

func TestLargeDictionaryMatchThreadSafeWorks(t *testing.T) {
	/**
	 * we have 105 unique words extracted from dictionary, therefore the result
	 * is supposed to show 105 hits
	 */
	hits := precomputed6.MatchThreadSafe(bytes2)
	assert(t, len(hits) == 105)

}

func TestMatchThreadSafePoolReuse(t *testing.T) {
	m := NewStringMatcher([]string{"Mozilla", "Mac", "Macintosh", "Safari"})
	in := []byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36")
	// First call populates the pool; second call reuses the pooled heap.
	hits := m.MatchThreadSafe(in)
	assert(t, len(hits) == 4)
	hits = m.MatchThreadSafe(in)
	assert(t, len(hits) == 4)
}

func TestNewMatcher(t *testing.T) {
	m := NewMatcher([][]byte{[]byte("Mozilla"), []byte("Mac"), []byte("Macintosh"), []byte("Safari")})
	hits := m.Match([]byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
	assert(t, hits[3] == 3)
}

func TestReuse(t *testing.T) {
	m := NewStringMatcher([]string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"})
	in := []byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36")
	check1 := func(t *testing.T, n int, hits []int) {
		t.Helper()
		assert(t, n == 4)
		assert(t, hits[0] == 0)
		assert(t, hits[1] == 1)
		assert(t, hits[2] == 2)
		assert(t, hits[3] == 3)
	}
	check2 := func(t *testing.T, n int, _ []int) {
		t.Helper()
		assert(t, n == 0)
	}

	t.Run("MatchInto", func(t *testing.T) {
		dst := make([]int, 0, 8)

		n := m.MatchInto(in, &dst)
		check1(t, n, dst[:n])

		// reuse dst — no new allocation
		dst = dst[:0]
		n = m.MatchInto([]byte("no match here"), &dst)
		check2(t, n, dst[:n])
	})

	t.Run("MatchThreadSafeInto", func(t *testing.T) {
		dst := make([]int, 0, 8)

		n := m.MatchThreadSafeInto(in, &dst)
		check1(t, n, dst[:n])

		// reuse dst — no new allocation
		dst = dst[:0]
		n = m.MatchThreadSafeInto([]byte("no match here"), &dst)
		check2(t, n, dst[:n])
	})
}

// TestIntoGrowth verifies that the Into variants correctly reallocate dst when
// the number of matches exceeds the initial capacity.
func TestIntoGrowth(t *testing.T) {
	// Build a matcher where a single input triggers more matches than the
	// initial dst capacity so that append must reallocate.
	dict := []string{"a", "b", "c", "d", "e"}
	m := NewStringMatcher(dict)
	in := []byte("abcde")

	checkIndices := func(t *testing.T, n int, got []int) {
		t.Helper()
		assert(t, n == 5)
		assert(t, len(got) == 5)
		for i, v := range got {
			assert(t, v == i)
		}
	}

	checkPositions := func(t *testing.T, n int, got []Position) {
		t.Helper()
		assert(t, n == 5)
		for i, p := range got {
			assert(t, p.Index == i)
			assert(t, p.Start == i)
			assert(t, p.End == i+1)
		}
	}

	t.Run("MatchInto", func(t *testing.T) {
		dst := make([]int, 0, 1) // capacity 1, but 5 matches expected
		n := m.MatchInto(in, &dst)
		checkIndices(t, n, dst[:n])
	})

	t.Run("MatchThreadSafeInto", func(t *testing.T) {
		dst := make([]int, 0, 1)
		n := m.MatchThreadSafeInto(in, &dst)
		checkIndices(t, n, dst[:n])
	})

	t.Run("MatchPositionsInto", func(t *testing.T) {
		dst := make([]Position, 0, 1)
		n := m.MatchPositionsInto(in, &dst)
		checkPositions(t, n, dst[:n])
	})

	t.Run("MatchPositionsThreadSafeInto", func(t *testing.T) {
		dst := make([]Position, 0, 1)
		n := m.MatchPositionsThreadSafeInto(in, &dst)
		checkPositions(t, n, dst[:n])
	})
}

// FuzzMatch verifies all match methods agree and are correct against a fixed
// dictionary. It checks:
//   - all four methods return the same hit indices (not just counts)
//   - Contains agrees with whether any hits were found
//   - no pattern present via linear scan is missed (no false negatives)
//   - no reported hit refers to a pattern absent from the input (no false positives)
func FuzzMatch(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(""),
		[]byte("Mozilla"),
		[]byte("The Man Of Steel: Superman"),
		[]byte("abccab"),
	} {
		f.Add(seed)
	}
	dict := []string{"Mozilla", "Mac", "Macintosh", "Safari", "Superman", "ab", "a"}
	m := NewStringMatcher(dict)
	dictBytes := make([][]byte, len(dict))
	for i, s := range dict {
		dictBytes[i] = []byte(s)
	}
	dstInto := make([]int, 0, 8)
	dstTSInto := make([]int, 0, 8)
	f.Fuzz(func(t *testing.T, in []byte) {
		hits := m.Match(in)
		hitsTS := m.MatchThreadSafe(in)
		contains := m.Contains(in)

		dstInto = dstInto[:0]
		nInto := m.MatchInto(in, &dstInto)
		hitsInto := dstInto[:nInto]

		dstTSInto = dstTSInto[:0]
		nTSInto := m.MatchThreadSafeInto(in, &dstTSInto)
		hitsTSInto := dstTSInto[:nTSInto]

		// All four methods must return identical hit sets.
		toSet := func(s []int) map[int]bool {
			m := make(map[int]bool, len(s))
			for _, v := range s {
				m[v] = true
			}
			return m
		}
		hitSet := toSet(hits)
		if !reflect.DeepEqual(hitSet, toSet(hitsTS)) {
			t.Fatalf("Match=%v but MatchThreadSafe=%v for input %q", hits, hitsTS, in)
		}
		if !reflect.DeepEqual(hitSet, toSet(hitsInto)) {
			t.Fatalf("Match=%v but MatchInto=%v for input %q", hits, hitsInto, in)
		}
		if !reflect.DeepEqual(hitSet, toSet(hitsTSInto)) {
			t.Fatalf("Match=%v but MatchThreadSafeInto=%v for input %q", hits, hitsTSInto, in)
		}

		// Contains must agree with whether any hits were found.
		if contains != (len(hits) > 0) {
			t.Fatalf("Contains=%v but Match found %d hits for input %q", contains, len(hits), in)
		}

		// Oracle: no false negatives — every pattern present in input must be hit.
		// Oracle: no false positives — every hit must refer to a pattern in input.
		for i, p := range dictBytes {
			inInput := bytespkg.Contains(in, p)
			wasHit := hitSet[i]
			if inInput && !wasHit {
				t.Fatalf("false negative: missed pattern[%d]=%q in %q", i, p, in)
			}
			if !inInput && wasHit {
				t.Fatalf("false positive: reported pattern[%d]=%q not in %q", i, p, in)
			}
		}
	})
}

// FuzzNewMatcher verifies the matcher doesn't panic for arbitrary dictionaries
// and inputs, and that every pattern present via linear scan is found by the
// automaton (oracle check).
//
// The entire corpus is encoded as a single []byte with fields separated by
// \x00: the last field is the input to match; all preceding fields are
// patterns. This lets the fuzzer simultaneously vary the number of patterns,
// their content, and the input string in a single mutation pass.
//
// Individual pattern length is capped: buildTrie's suffix-finding BFS is
// O(n²) in pattern length, so unbounded lengths cause per-input timeouts.
// Pattern count is capped at 519 (the original real-world dictionary size).
func FuzzNewMatcher(f *testing.F) {
	// encode packs patterns and input into a \x00-delimited blob.
	encode := func(patterns []string, input string) []byte {
		var buf []byte
		for _, p := range patterns {
			buf = append(buf, []byte(p)...)
			buf = append(buf, 0)
		}
		buf = append(buf, []byte(input)...)
		return buf
	}
	f.Add(encode([]string{"foo", "bar"}, "hello world"))
	f.Add(encode([]string{"a", "ab"}, "abc"))
	f.Add(encode([]string{"x"}, ""))
	// Seed encoding the structural condition that caused the bug:
	// a pattern whose prefix matches the suffix of an earlier pattern,
	// with enough preceding patterns to push nodes to higher trie indices.
	f.Add(encode([]string{
		"Googlebot-Mobile", "Googlebot-Image", "Googlebot-News", "Googlebot-Video",
		"AdsBot-Google-Mobile", "Google-Ads-Conversions", "Feedfetcher-Google",
		"Mediapartners-Google", "APIs-Google", "Google-InspectionTool",
		"Storebot-Google", "GoogleOther", "bingbot", "Slurp", "LinkedInBot",
		"Python-urllib", "python-requests", "aiohttp", "httpx",
	}, "python-httpx/0.16.1"))

	f.Fuzz(func(t *testing.T, blob []byte) {
		const maxPatternLen = 128
		const maxPatterns = 519

		// Split on \x00: all but the last segment are patterns; last is input.
		parts := bytespkg.Split(blob, []byte{0})
		if len(parts) < 2 {
			t.Skip() // need at least one pattern and one input
		}
		patternParts := parts[:len(parts)-1]
		in := parts[len(parts)-1]

		if len(patternParts) > maxPatterns {
			t.Skip()
		}
		dict := make([][]byte, 0, len(patternParts))
		for _, p := range patternParts {
			if len(p) > maxPatternLen {
				t.Skip()
			}
			dict = append(dict, p)
		}

		m := NewMatcher(dict)
		hits := m.Match(in)
		hitsTS := m.MatchThreadSafe(in)
		contains := m.Contains(in)

		dstInto := make([]int, 0, len(dict))
		nInto := m.MatchInto(in, &dstInto)
		hitsInto := dstInto[:nInto]

		dstTSInto := make([]int, 0, len(dict))
		nTSInto := m.MatchThreadSafeInto(in, &dstTSInto)
		hitsTSInto := dstTSInto[:nTSInto]

		// All four methods must return identical hit sets.
		toSet := func(s []int) map[int]bool {
			m := make(map[int]bool, len(s))
			for _, v := range s {
				m[v] = true
			}
			return m
		}
		hitSet := toSet(hits)
		if !reflect.DeepEqual(hitSet, toSet(hitsTS)) {
			t.Fatalf("Match=%v but MatchThreadSafe=%v for input %q dict=%q", hits, hitsTS, in, dict)
		}
		if !reflect.DeepEqual(hitSet, toSet(hitsInto)) {
			t.Fatalf("Match=%v but MatchInto=%v for input %q dict=%q", hits, hitsInto, in, dict)
		}
		if !reflect.DeepEqual(hitSet, toSet(hitsTSInto)) {
			t.Fatalf("Match=%v but MatchThreadSafeInto=%v for input %q dict=%q", hits, hitsTSInto, in, dict)
		}
		if contains != (len(hits) > 0) {
			t.Fatalf("Contains=%v but Match found %d hits for input %q dict=%q", contains, len(hits), in, dict)
		}

		// Oracle: every distinct pattern present via linear scan must be reported
		// (no false negatives), and every reported hit must be present in the input
		// (no false positives).
		// For duplicate patterns (same bytes at different indices), the trie stores
		// the last index, so we check by pattern content rather than index.
		hitPatterns := make(map[string]bool, len(hits))
		for _, i := range hits {
			hitPatterns[string(dict[i])] = true
		}
		for _, p := range dict {
			if len(p) == 0 {
				continue
			}
			inInput := bytespkg.Contains(in, p)
			wasHit := hitPatterns[string(p)]
			if inInput && !wasHit {
				t.Fatalf("false negative: missed pattern %q in %q dict=%q", p, in, dict)
			}
			if !inInput && wasHit {
				t.Fatalf("false positive: reported pattern %q not in %q dict=%q", p, in, dict)
			}
		}
	})
}

// FuzzNewMatcherPositions verifies the MatchPositions* family against arbitrary
// dictionaries and inputs. Unlike FuzzNewMatcher it does not deduplicate: every
// individual occurrence of every pattern must be reported with correct
// [Start, End) byte offsets.
//
// The oracle scans the input with bytes.Index to enumerate all occurrences of
// each pattern, then compares against the automaton's output position-by-position.
//
// Encoding and size limits are identical to FuzzNewMatcher.
func FuzzNewMatcherPositions(f *testing.F) {
	encode := func(patterns []string, input string) []byte {
		var buf []byte
		for _, p := range patterns {
			buf = append(buf, []byte(p)...)
			buf = append(buf, 0)
		}
		buf = append(buf, []byte(input)...)
		return buf
	}
	f.Add(encode([]string{"foo", "bar"}, "hello world"))
	f.Add(encode([]string{"a", "ab"}, "abc"))
	f.Add(encode([]string{"x"}, ""))
	f.Add(encode([]string{"Superman", "uperman", "perman"}, "The Man Of Steel: Superman"))
	f.Add(encode([]string{"a", "ab", "bc", "bca", "c", "caa"}, "abccab"))

	f.Fuzz(func(t *testing.T, blob []byte) {
		const maxPatternLen = 128
		const maxPatterns = 519

		parts := bytespkg.Split(blob, []byte{0})
		if len(parts) < 2 {
			t.Skip()
		}
		patternParts := parts[:len(parts)-1]
		in := parts[len(parts)-1]

		if len(patternParts) > maxPatterns {
			t.Skip()
		}
		dict := make([][]byte, 0, len(patternParts))
		for _, p := range patternParts {
			if len(p) > maxPatternLen {
				t.Skip()
			}
			dict = append(dict, p)
		}

		m := NewMatcher(dict)

		positions := m.MatchPositions(in)
		positionsTS := m.MatchPositionsThreadSafe(in)

		dstInto := make([]Position, 0, len(dict))
		nInto := m.MatchPositionsInto(in, &dstInto)
		positionsInto := dstInto[:nInto]

		dstTSInto := make([]Position, 0, len(dict))
		nTSInto := m.MatchPositionsThreadSafeInto(in, &dstTSInto)
		positionsTSInto := dstTSInto[:nTSInto]

		// Normalise nil vs empty so reflect.DeepEqual doesn't diverge on
		// slices that are logically identical but one is nil and the other [].
		normalise := func(s []Position) []Position {
			if s == nil {
				return []Position{}
			}
			return s
		}
		positions = normalise(positions)
		positionsTS = normalise(positionsTS)
		positionsInto = normalise(positionsInto)
		positionsTSInto = normalise(positionsTSInto)

		// All four methods must return identical position slices.
		if !reflect.DeepEqual(positions, positionsTS) {
			t.Fatalf("MatchPositions=%v but MatchPositionsThreadSafe=%v for input %q dict=%q", positions, positionsTS, in, dict)
		}
		if !reflect.DeepEqual(positions, positionsInto) {
			t.Fatalf("MatchPositions=%v but MatchPositionsInto=%v for input %q dict=%q", positions, positionsInto, in, dict)
		}
		if !reflect.DeepEqual(positions, positionsTSInto) {
			t.Fatalf("MatchPositions=%v but MatchPositionsThreadSafeInto=%v for input %q dict=%q", positions, positionsTSInto, in, dict)
		}

		// Oracle: build the expected set of (index, start, end) occurrences by
		// scanning each pattern with bytes.Index. Duplicate patterns share bytes
		// but have different indices; the trie stores the last index for a given
		// byte sequence, so we track which index the automaton will use per unique
		// byte sequence.
		//
		// Because duplicate-content patterns map to a single trie node (last-writer
		// wins on index), we resolve the canonical index for each pattern content.
		canonicalIndex := make(map[string]int, len(dict))
		for i, p := range dict {
			if len(p) > 0 {
				canonicalIndex[string(p)] = i
			}
		}

		type posKey struct {
			index      int
			start, end int
		}
		expected := make(map[posKey]bool)
		for _, p := range dict {
			if len(p) == 0 {
				continue
			}
			ci := canonicalIndex[string(p)]
			offset := 0
			for {
				idx := bytespkg.Index(in[offset:], p)
				if idx < 0 {
					break
				}
				start := offset + idx
				end := start + len(p)
				expected[posKey{ci, start, end}] = true
				offset = start + 1
			}
		}

		got := make(map[posKey]bool, len(positions))
		for _, pos := range positions {
			// Validate the reported bytes match the pattern.
			if pos.Start < 0 || pos.End > len(in) || pos.Start > pos.End {
				t.Fatalf("position out of bounds: %+v for input %q dict=%q", pos, in, dict)
			}
			if pos.Index < 0 || pos.Index >= len(dict) {
				t.Fatalf("index out of range: %+v for input %q dict=%q", pos, in, dict)
			}
			matched := in[pos.Start:pos.End]
			if !bytespkg.Equal(matched, dict[pos.Index]) {
				t.Fatalf("position mismatch: pos=%+v matched=%q but dict[%d]=%q for input %q dict=%q",
					pos, matched, pos.Index, dict[pos.Index], in, dict)
			}
			got[posKey{pos.Index, pos.Start, pos.End}] = true
		}

		// No false negatives.
		for k := range expected {
			if !got[k] {
				t.Fatalf("false negative: missing position {index=%d start=%d end=%d} for input %q dict=%q",
					k.index, k.start, k.end, in, dict)
			}
		}
		// No false positives.
		for k := range got {
			if !expected[k] {
				t.Fatalf("false positive: unexpected position {index=%d start=%d end=%d} for input %q dict=%q",
					k.index, k.start, k.end, in, dict)
			}
		}
	})
}

// FuzzMatchThreadSafeConcurrent verifies MatchThreadSafe is safe under
// concurrent use. Run with -race to catch data races.
func FuzzMatchThreadSafeConcurrent(f *testing.F) {
	encode := func(patterns []string, input string) []byte {
		var buf []byte
		for _, p := range patterns {
			buf = append(buf, []byte(p)...)
			buf = append(buf, 0)
		}
		buf = append(buf, []byte(input)...)
		return buf
	}
	f.Add(encode([]string{"foo", "bar"}, "foo bar baz"))
	f.Add(encode([]string{"Mozilla", "Mac", "Safari"}, "Mozilla/5.0 (Macintosh)"))
	f.Fuzz(func(t *testing.T, blob []byte) {
		const maxPatternLen = 64
		const maxPatterns = 16
		parts := bytespkg.Split(blob, []byte{0})
		if len(parts) < 2 || len(parts)-1 > maxPatterns {
			t.Skip()
		}
		dict := make([][]byte, 0, len(parts)-1)
		for _, p := range parts[:len(parts)-1] {
			if len(p) > maxPatternLen {
				t.Skip()
			}
			dict = append(dict, p)
		}
		in := parts[len(parts)-1]

		m := NewMatcher(dict)
		// Run several concurrent MatchThreadSafe calls on the same matcher.
		// The race detector will flag any unsynchronised access.
		const workers = 4
		results := make([][]int, workers)
		var wg sync.WaitGroup
		wg.Add(workers)
		for i := range workers {
			go func(i int) {
				defer wg.Done()
				results[i] = m.MatchThreadSafe(in)
			}(i)
		}
		wg.Wait()
		// All goroutines must have seen the same result.
		for i := 1; i < workers; i++ {
			if !reflect.DeepEqual(results[0], results[i]) {
				t.Fatalf("concurrent MatchThreadSafe disagreement: goroutine 0=%v goroutine %d=%v for input %q dict=%q",
					results[0], i, results[i], in, dict)
			}
		}
	})
}

var bytes = []byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36")
var sbytes = string(bytes)
var dictionary = []string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"}
var precomputed = NewStringMatcher(dictionary)
var re = regexp.MustCompile("(" + strings.Join(dictionary, "|") + ")")

func BenchmarkWorks(b *testing.B) {
	b.Run("Match", func(b *testing.B) {
		for b.Loop() {
			precomputed.Match(bytes)
		}
	})
	b.Run("MatchThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed.MatchThreadSafe(bytes)
		}
	})
	b.Run("MatchThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed.MatchThreadSafe(bytes)
			}
		})
	})
	b.Run("MatchInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed.MatchInto(bytes, &dst)
		}
	})
	b.Run("MatchThreadSafeInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed.MatchThreadSafeInto(bytes, &dst)
		}
	})
	b.Run("MatchThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]int, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed.MatchThreadSafeInto(bytes, &dst)
			}
		})
	})
	b.Run("MatchPositions", func(b *testing.B) {
		for b.Loop() {
			precomputed.MatchPositions(bytes)
		}
	})
	b.Run("MatchPositionsThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed.MatchPositionsThreadSafe(bytes)
		}
	})
	b.Run("MatchPositionsThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed.MatchPositionsThreadSafe(bytes)
			}
		})
	})
	b.Run("MatchPositionsInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed.MatchPositionsInto(bytes, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed.MatchPositionsThreadSafeInto(bytes, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]Position, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed.MatchPositionsThreadSafeInto(bytes, &dst)
			}
		})
	})
	b.Run("Contains", func(b *testing.B) {
		for b.Loop() {
			precomputed.Contains(bytes)
		}
	})
	b.Run("StringsContains", func(b *testing.B) {
		for b.Loop() {
			hits := make([]int, 0)
			for i, s := range dictionary {
				if strings.Contains(sbytes, s) {
					hits = append(hits, i)
				}
			}
			_ = hits
		}
	})
	b.Run("Regexp", func(b *testing.B) {
		for b.Loop() {
			re.FindAllIndex(bytes, -1)
		}
	})
}

var dictionary2 = []string{"Googlebot", "bingbot", "msnbot", "Yandex", "Baiduspider"}
var precomputed2 = NewStringMatcher(dictionary2)
var re2 = regexp.MustCompile("(" + strings.Join(dictionary2, "|") + ")")

func BenchmarkFails(b *testing.B) {
	b.Run("Match", func(b *testing.B) {
		for b.Loop() {
			precomputed2.Match(bytes)
		}
	})
	b.Run("MatchThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed2.MatchThreadSafe(bytes)
		}
	})
	b.Run("MatchThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed2.MatchThreadSafe(bytes)
			}
		})
	})
	b.Run("MatchInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed2.MatchInto(bytes, &dst)
		}
	})
	b.Run("MatchThreadSafeInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed2.MatchThreadSafeInto(bytes, &dst)
		}
	})
	b.Run("MatchThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]int, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed2.MatchThreadSafeInto(bytes, &dst)
			}
		})
	})
	b.Run("MatchPositions", func(b *testing.B) {
		for b.Loop() {
			precomputed2.MatchPositions(bytes)
		}
	})
	b.Run("MatchPositionsThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed2.MatchPositionsThreadSafe(bytes)
		}
	})
	b.Run("MatchPositionsThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed2.MatchPositionsThreadSafe(bytes)
			}
		})
	})
	b.Run("MatchPositionsInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed2.MatchPositionsInto(bytes, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed2.MatchPositionsThreadSafeInto(bytes, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]Position, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed2.MatchPositionsThreadSafeInto(bytes, &dst)
			}
		})
	})
	b.Run("Contains", func(b *testing.B) {
		for b.Loop() {
			precomputed2.Contains(bytes)
		}
	})
	b.Run("StringsContains", func(b *testing.B) {
		for b.Loop() {
			hits := make([]int, 0)
			for i, s := range dictionary2 {
				if strings.Contains(sbytes, s) {
					hits = append(hits, i)
				}
			}
			_ = hits
		}
	})
	b.Run("Regexp", func(b *testing.B) {
		for b.Loop() {
			re2.FindAllIndex(bytes, -1)
		}
	})
}

var bytes2 = []byte("Firefox is a web browser, and is Mozilla's flagship software product. It is available in both desktop and mobile versions. Firefox uses the Gecko layout engine to render web pages, which implements current and anticipated web standards. As of April 2013, Firefox has approximately 20% of worldwide usage share of web browsers, making it the third most-used web browser. Firefox began as an experimental branch of the Mozilla codebase by Dave Hyatt, Joe Hewitt and Blake Ross. They believed the commercial requirements of Netscape's sponsorship and developer-driven feature creep compromised the utility of the Mozilla browser. To combat what they saw as the Mozilla Suite's software bloat, they created a stand-alone browser, with which they intended to replace the Mozilla Suite. Firefox was originally named Phoenix but the name was changed so as to avoid trademark conflicts with Phoenix Technologies. The initially-announced replacement, Firebird, provoked objections from the Firebird project community. The current name, Firefox, was chosen on February 9, 2004.")
var sbytes2 = string(bytes2)
var dictionary3 = []string{"Mozilla", "Mac", "Macintosh", "Safari", "Phoenix"}
var precomputed3 = NewStringMatcher(dictionary3)
var re3 = regexp.MustCompile("(" + strings.Join(dictionary3, "|") + ")")

func BenchmarkLongWorks(b *testing.B) {
	b.Run("Match", func(b *testing.B) {
		for b.Loop() {
			precomputed3.Match(bytes2)
		}
	})
	b.Run("MatchThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed3.MatchThreadSafe(bytes2)
		}
	})
	b.Run("MatchThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed3.MatchThreadSafe(bytes2)
			}
		})
	})
	b.Run("MatchInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed3.MatchInto(bytes2, &dst)
		}
	})
	b.Run("MatchThreadSafeInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed3.MatchThreadSafeInto(bytes2, &dst)
		}
	})
	b.Run("MatchThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]int, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed3.MatchThreadSafeInto(bytes2, &dst)
			}
		})
	})
	b.Run("MatchPositions", func(b *testing.B) {
		for b.Loop() {
			precomputed3.MatchPositions(bytes2)
		}
	})
	b.Run("MatchPositionsThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed3.MatchPositionsThreadSafe(bytes2)
		}
	})
	b.Run("MatchPositionsThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed3.MatchPositionsThreadSafe(bytes2)
			}
		})
	})
	b.Run("MatchPositionsInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed3.MatchPositionsInto(bytes2, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed3.MatchPositionsThreadSafeInto(bytes2, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]Position, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed3.MatchPositionsThreadSafeInto(bytes2, &dst)
			}
		})
	})
	b.Run("Contains", func(b *testing.B) {
		for b.Loop() {
			precomputed3.Contains(bytes2)
		}
	})
	b.Run("StringsContains", func(b *testing.B) {
		for b.Loop() {
			hits := make([]int, 0)
			for i, s := range dictionary3 {
				if strings.Contains(sbytes2, s) {
					hits = append(hits, i)
				}
			}
			_ = hits
		}
	})
	b.Run("Regexp", func(b *testing.B) {
		for b.Loop() {
			re3.FindAllIndex(bytes2, -1)
		}
	})
}

var dictionary4 = []string{"12343453", "34353", "234234523", "324234", "33333"}
var precomputed4 = NewStringMatcher(dictionary4)
var re4 = regexp.MustCompile("(" + strings.Join(dictionary4, "|") + ")")

func BenchmarkLongFails(b *testing.B) {
	b.Run("Match", func(b *testing.B) {
		for b.Loop() {
			precomputed4.Match(bytes2)
		}
	})
	b.Run("MatchThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed4.MatchThreadSafe(bytes2)
		}
	})
	b.Run("MatchThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed4.MatchThreadSafe(bytes2)
			}
		})
	})
	b.Run("MatchInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed4.MatchInto(bytes2, &dst)
		}
	})
	b.Run("MatchThreadSafeInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed4.MatchThreadSafeInto(bytes2, &dst)
		}
	})
	b.Run("MatchThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]int, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed4.MatchThreadSafeInto(bytes2, &dst)
			}
		})
	})
	b.Run("MatchPositions", func(b *testing.B) {
		for b.Loop() {
			precomputed4.MatchPositions(bytes2)
		}
	})
	b.Run("MatchPositionsThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed4.MatchPositionsThreadSafe(bytes2)
		}
	})
	b.Run("MatchPositionsThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed4.MatchPositionsThreadSafe(bytes2)
			}
		})
	})
	b.Run("MatchPositionsInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed4.MatchPositionsInto(bytes2, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed4.MatchPositionsThreadSafeInto(bytes2, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]Position, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed4.MatchPositionsThreadSafeInto(bytes2, &dst)
			}
		})
	})
	b.Run("Contains", func(b *testing.B) {
		for b.Loop() {
			precomputed4.Contains(bytes2)
		}
	})
	b.Run("StringsContains", func(b *testing.B) {
		for b.Loop() {
			hits := make([]int, 0)
			for i, s := range dictionary4 {
				if strings.Contains(sbytes2, s) {
					hits = append(hits, i)
				}
			}
			_ = hits
		}
	})
	b.Run("Regexp", func(b *testing.B) {
		for b.Loop() {
			re4.FindAllIndex(bytes2, -1)
		}
	})
}

var dictionary5 = []string{"12343453", "34353", "234234523", "324234", "33333", "experimental", "branch", "of", "the", "Mozilla", "codebase", "by", "Dave", "Hyatt", "Joe", "Hewitt", "and", "Blake", "Ross", "mother", "frequently", "performed", "in", "concerts", "around", "the", "village", "uses", "the", "Gecko", "layout", "engine"}
var precomputed5 = NewStringMatcher(dictionary5)
var re5 = regexp.MustCompile("(" + strings.Join(dictionary5, "|") + ")")

func BenchmarkMany(b *testing.B) {
	b.Run("Match", func(b *testing.B) {
		for b.Loop() {
			precomputed5.Match(bytes2)
		}
	})
	b.Run("MatchThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed5.MatchThreadSafe(bytes2)
		}
	})
	b.Run("MatchThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed5.MatchThreadSafe(bytes2)
			}
		})
	})
	b.Run("MatchInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed5.MatchInto(bytes2, &dst)
		}
	})
	b.Run("MatchThreadSafeInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed5.MatchThreadSafeInto(bytes2, &dst)
		}
	})
	b.Run("MatchThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]int, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed5.MatchThreadSafeInto(bytes2, &dst)
			}
		})
	})
	b.Run("MatchPositions", func(b *testing.B) {
		for b.Loop() {
			precomputed5.MatchPositions(bytes2)
		}
	})
	b.Run("MatchPositionsThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed5.MatchPositionsThreadSafe(bytes2)
		}
	})
	b.Run("MatchPositionsThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed5.MatchPositionsThreadSafe(bytes2)
			}
		})
	})
	b.Run("MatchPositionsInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed5.MatchPositionsInto(bytes2, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed5.MatchPositionsThreadSafeInto(bytes2, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]Position, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed5.MatchPositionsThreadSafeInto(bytes2, &dst)
			}
		})
	})
	b.Run("Contains", func(b *testing.B) {
		for b.Loop() {
			precomputed5.Contains(bytes2)
		}
	})
	b.Run("StringsContains", func(b *testing.B) {
		for b.Loop() {
			hits := make([]int, 0)
			for i, s := range dictionary5 {
				if strings.Contains(sbytes, s) {
					hits = append(hits, i)
				}
			}
			_ = hits
		}
	})
	b.Run("Regexp", func(b *testing.B) {
		for b.Loop() {
			re5.FindAllIndex(bytes2, -1)
		}
	})
}

var dictionary6 = []string{"2004", "2013", "9", "a", "an", "and", "anticipated", "approximately", "April", "as", "available", "avoid", "began", "believed", "Blake", "bloat", "both", "branch", "browser", "browsers", "but", "by", "changed", "chosen", "codebase", "combat", "commercial", "community", "compromised", "conflicts", "created", "creep", "current", "Dave", "desktop", "developer-driven", "engine", "experimental", "feature", "February", "Firebird", "Firefox", "flagship", "from", "Gecko", "has", "Hewitt", "Hyatt", "implements", "in", "initially-announced", "intended", "is", "it", "Joe", "layout", "making", "mobile", "most-used", "Mozilla", "Mozilla's", "name", "named", "Netscape's", "objections", "of", "on", "originally", "pages", "Phoenix", "product", "project", "provoked", "render", "replace", "replacement", "requirements", "Ross", "saw", "share", "so", "software", "sponsorship", "stand-alone", "standards", "Suite", "Suite's", "Technologies", "the", "The", "they", "They", "third", "to", "trademark", "usage", "uses", "utility", "versions", "was", "web", "what", "which", "with", "worldwide"}
var precomputed6 = NewStringMatcher(dictionary6)
var re6 = regexp.MustCompile("(" + strings.Join(dictionary6, "|") + ")")

func BenchmarkLargeWorks(b *testing.B) {
	b.Run("Match", func(b *testing.B) {
		for b.Loop() {
			precomputed6.Match(bytes2)
		}
	})
	b.Run("MatchThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed6.MatchThreadSafe(bytes2)
		}
	})
	b.Run("MatchThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed6.MatchThreadSafe(bytes2)
			}
		})
	})
	b.Run("MatchInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed6.MatchInto(bytes2, &dst)
		}
	})
	b.Run("MatchThreadSafeInto", func(b *testing.B) {
		dst := make([]int, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed6.MatchThreadSafeInto(bytes2, &dst)
		}
	})
	b.Run("MatchThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]int, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed6.MatchThreadSafeInto(bytes2, &dst)
			}
		})
	})
	b.Run("MatchPositions", func(b *testing.B) {
		for b.Loop() {
			precomputed6.MatchPositions(bytes2)
		}
	})
	b.Run("MatchPositionsThreadSafe", func(b *testing.B) {
		for b.Loop() {
			precomputed6.MatchPositionsThreadSafe(bytes2)
		}
	})
	b.Run("MatchPositionsThreadSafeConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				precomputed6.MatchPositionsThreadSafe(bytes2)
			}
		})
	})
	b.Run("MatchPositionsInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed6.MatchPositionsInto(bytes2, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeInto", func(b *testing.B) {
		dst := make([]Position, 0, 8)
		for b.Loop() {
			dst = dst[:0]
			precomputed6.MatchPositionsThreadSafeInto(bytes2, &dst)
		}
	})
	b.Run("MatchPositionsThreadSafeIntoConcurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			dst := make([]Position, 0, 8)
			for pb.Next() {
				dst = dst[:0]
				precomputed6.MatchPositionsThreadSafeInto(bytes2, &dst)
			}
		})
	})
	b.Run("Contains", func(b *testing.B) {
		for b.Loop() {
			precomputed6.Contains(bytes2)
		}
	})
	b.Run("StringsContains", func(b *testing.B) {
		for b.Loop() {
			hits := make([]int, 0)
			for i, s := range dictionary6 {
				if strings.Contains(sbytes, s) {
					hits = append(hits, i)
				}
			}
			_ = hits
		}
	})
	b.Run("Regexp", func(b *testing.B) {
		for b.Loop() {
			re6.FindAllIndex(bytes2, -1)
		}
	})
}

func BenchmarkNewStringMatcher(b *testing.B) {
	b.Run("Small", func(b *testing.B) {
		for b.Loop() {
			NewStringMatcher(dictionary)
		}
	})
	b.Run("Large", func(b *testing.B) {
		for b.Loop() {
			NewStringMatcher(dictionary6)
		}
	})
}
