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

func TestNoPatterns(t *testing.T) {
	m := NewStringMatcher([]string{})
	hits := m.Match([]byte("foo bar baz"))
	assert(t, len(hits) == 0)

	hits = m.MatchThreadSafe([]byte("foo bar baz"))
	assert(t, len(hits) == 0)
}

func TestNoData(t *testing.T) {
	m := NewStringMatcher([]string{"foo", "baz", "bar"})
	hits := m.Match([]byte(""))
	assert(t, len(hits) == 0)

	hits = m.MatchThreadSafe([]byte(""))
	assert(t, len(hits) == 0)
}

func TestSuffixes(t *testing.T) {
	m := NewStringMatcher([]string{"Superman", "uperman", "perman", "erman"})
	hits := m.Match([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
	assert(t, hits[3] == 3)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
	assert(t, hits[3] == 3)
}

func TestPrefixes(t *testing.T) {
	m := NewStringMatcher([]string{"Superman", "Superma", "Superm", "Super"})
	hits := m.Match([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 3)
	assert(t, hits[1] == 2)
	assert(t, hits[2] == 1)
	assert(t, hits[3] == 0)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 3)
	assert(t, hits[1] == 2)
	assert(t, hits[2] == 1)
	assert(t, hits[3] == 0)
}

func TestInterior(t *testing.T) {
	m := NewStringMatcher([]string{"Steel", "tee", "e"})
	hits := m.Match([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 3)
	assert(t, hits[2] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[0] == 2)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 3)
	assert(t, hits[2] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[0] == 2)
}

func TestMatchAtStart(t *testing.T) {
	m := NewStringMatcher([]string{"The", "Th", "he"})
	hits := m.Match([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 0)
	assert(t, hits[2] == 2)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel: Superman"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 0)
	assert(t, hits[2] == 2)
}

func TestMatchAtEnd(t *testing.T) {
	m := NewStringMatcher([]string{"teel", "eel", "el"})
	hits := m.Match([]byte("The Man Of Steel"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
}

func TestOverlappingPatterns(t *testing.T) {
	m := NewStringMatcher([]string{"Man ", "n Of", "Of S"})
	hits := m.Match([]byte("The Man Of Steel"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)

	hits = m.MatchThreadSafe([]byte("The Man Of Steel"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
}

func TestMultipleMatches(t *testing.T) {
	m := NewStringMatcher([]string{"The", "Man", "an"})
	hits := m.Match([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 2)
	assert(t, hits[2] == 0)

	hits = m.MatchThreadSafe([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 2)
	assert(t, hits[2] == 0)
}

func TestSingleCharacterMatches(t *testing.T) {
	m := NewStringMatcher([]string{"a", "M", "z"})
	hits := m.Match([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 2)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 0)

	hits = m.MatchThreadSafe([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 2)
	assert(t, hits[0] == 1)
	assert(t, hits[1] == 0)
}

func TestNothingMatches(t *testing.T) {
	m := NewStringMatcher([]string{"baz", "bar", "foo"})
	hits := m.Match([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 0)

	hits = m.MatchThreadSafe([]byte("A Man A Plan A Canal: Panama, which Man Planned The Canal"))
	assert(t, len(hits) == 0)
}

func TestWikipedia(t *testing.T) {
	m := NewStringMatcher([]string{"a", "ab", "bc", "bca", "c", "caa"})
	hits := m.Match([]byte("abccab"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
	assert(t, hits[3] == 4)

	hits = m.Match([]byte("bccab"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 2)
	assert(t, hits[1] == 4)
	assert(t, hits[2] == 0)
	assert(t, hits[3] == 1)

	hits = m.Match([]byte("bccb"))
	assert(t, len(hits) == 2)
	assert(t, hits[0] == 2)
	assert(t, hits[1] == 4)
}

func TestWikipediaConcurrently(t *testing.T) {
	m := NewStringMatcher([]string{"a", "ab", "bc", "bca", "c", "caa"})

	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		hits := m.MatchThreadSafe([]byte("abccab"))
		assert(t, len(hits) == 4)
		assert(t, hits[0] == 0)
		assert(t, hits[1] == 1)
		assert(t, hits[2] == 2)
		assert(t, hits[3] == 4)
	}()

	go func() {
		defer wg.Done()
		hits := m.MatchThreadSafe([]byte("bccab"))
		assert(t, len(hits) == 4)
		assert(t, hits[0] == 2)
		assert(t, hits[1] == 4)
		assert(t, hits[2] == 0)
		assert(t, hits[3] == 1)
	}()

	go func() {
		defer wg.Done()
		hits := m.MatchThreadSafe([]byte("bccb"))
		assert(t, len(hits) == 2)
		assert(t, hits[0] == 2)
		assert(t, hits[1] == 4)
	}()

	wg.Wait()
}

func TestMatch(t *testing.T) {
	m := NewStringMatcher(dictionary)
	hits := m.Match([]byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
	assert(t, len(hits) == 4)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 2)
	assert(t, hits[3] == 3)

	hits = m.Match([]byte("Mozilla/5.0 (Mac; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
	assert(t, len(hits) == 3)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 1)
	assert(t, hits[2] == 3)

	hits = m.Match([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
	assert(t, len(hits) == 2)
	assert(t, hits[0] == 0)
	assert(t, hits[1] == 3)

	hits = m.Match([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
	assert(t, len(hits) == 1)
	assert(t, hits[0] == 0)

	hits = m.Match([]byte("Mazilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
	assert(t, len(hits) == 0)
}

func TestMatchThreadSafe(t *testing.T) {
	m := NewStringMatcher([]string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"})

	wg := sync.WaitGroup{}
	wg.Add(5)
	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafe([]byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
		assert(t, len(hits) == 4)
		assert(t, hits[0] == 0)
		assert(t, hits[1] == 1)
		assert(t, hits[2] == 2)
		assert(t, hits[3] == 3)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafe([]byte("Mozilla/5.0 (Mac; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
		assert(t, len(hits) == 3)
		assert(t, hits[0] == 0)
		assert(t, hits[1] == 1)
		assert(t, hits[2] == 3)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafe([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36"))
		assert(t, len(hits) == 2)
		assert(t, hits[0] == 0)
		assert(t, hits[1] == 3)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafe([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
		assert(t, len(hits) == 1)
		assert(t, hits[0] == 0)
	}()

	go func() {
		defer wg.Done()

		hits := m.MatchThreadSafe([]byte("Mazilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
		assert(t, len(hits) == 0)
	}()

	wg.Wait()
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

func TestMatchInto(t *testing.T) {
	m := NewStringMatcher([]string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"})
	dst := make([]int, 0, 8)

	in := []byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36")
	n := m.MatchInto(in, dst)
	assert(t, n == 4)
	dst = dst[:n]
	assert(t, dst[0] == 0)
	assert(t, dst[1] == 1)
	assert(t, dst[2] == 2)
	assert(t, dst[3] == 3)

	// reuse dst — no new allocation
	dst = dst[:0]
	n = m.MatchInto([]byte("no match here"), dst)
	assert(t, n == 0)
}

func TestMatchThreadSafeInto(t *testing.T) {
	m := NewStringMatcher([]string{"Mozilla", "Mac", "Macintosh", "Safari", "Sausage"})
	dst := make([]int, 0, 8)

	in := []byte("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Safari/537.36")
	n := m.MatchThreadSafeInto(in, dst)
	assert(t, n == 4)
	dst = dst[:n]
	assert(t, dst[0] == 0)
	assert(t, dst[1] == 1)
	assert(t, dst[2] == 2)
	assert(t, dst[3] == 3)

	// reuse dst and verify pool reuse path
	dst = dst[:0]
	n = m.MatchThreadSafeInto(in, dst)
	assert(t, n == 4)
}

func TestContains(t *testing.T) {
	m := NewStringMatcher(dictionary)
	contains := m.Contains([]byte("Mozilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
	assert(t, contains)

	contains = m.Contains([]byte("Mazilla/5.0 (Moc; Intel Computer OS X 10_7_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.101 Sofari/537.36"))
	assert(t, !contains)

	m = NewStringMatcher([]string{"SupermanX", "per"})
	contains = m.Contains([]byte("The Man Of Steel: Superman"))
	assert(t, contains == true)
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
		nInto := m.MatchInto(in, dstInto)
		hitsInto := dstInto[:nInto]

		dstTSInto = dstTSInto[:0]
		nTSInto := m.MatchThreadSafeInto(in, dstTSInto)
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
		nInto := m.MatchInto(in, dstInto)
		hitsInto := dstInto[:nInto]

		dstTSInto := make([]int, 0, len(dict))
		nTSInto := m.MatchThreadSafeInto(in, dstTSInto)
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

func BenchmarkMatchWorks(b *testing.B) {
	for b.Loop() {
		precomputed.Match(bytes)
	}
}

func BenchmarkMatchIntoWorks(b *testing.B) {
	dst := make([]int, 0, 8)
	for b.Loop() {
		dst = dst[:0]
		precomputed.MatchInto(bytes, dst)
	}
}

func BenchmarkMatchThreadSafeWorks(b *testing.B) {
	for b.Loop() {
		precomputed.MatchThreadSafe(bytes)
	}
}

func BenchmarkMatchThreadSafeIntoWorks(b *testing.B) {
	dst := make([]int, 0, 8)
	for b.Loop() {
		dst = dst[:0]
		precomputed.MatchThreadSafeInto(bytes, dst)
	}
}

func BenchmarkContainsWorks(b *testing.B) {
	for b.Loop() {
		precomputed.Contains(bytes)
	}
}

func BenchmarkStringsContainsWorks(b *testing.B) {
	for b.Loop() {
		hits := make([]int, 0)
		for i, s := range dictionary {
			if strings.Contains(sbytes, s) {
				hits = append(hits, i)
			}
		}
		_ = hits
	}
}

var re = regexp.MustCompile("(" + strings.Join(dictionary, "|") + ")")

func BenchmarkRegexpWorks(b *testing.B) {
	for b.Loop() {
		re.FindAllIndex(bytes, -1)
	}
}

var dictionary2 = []string{"Googlebot", "bingbot", "msnbot", "Yandex", "Baiduspider"}
var precomputed2 = NewStringMatcher(dictionary2)

func BenchmarkMatchFails(b *testing.B) {
	for b.Loop() {
		precomputed2.Match(bytes)
	}
}

func BenchmarkContainsFails(b *testing.B) {
	for b.Loop() {
		precomputed2.Contains(bytes)
	}
}

func BenchmarkStringsContainsFails(b *testing.B) {
	for b.Loop() {
		hits := make([]int, 0)
		for i, s := range dictionary2 {
			if strings.Contains(sbytes, s) {
				hits = append(hits, i)
			}
		}
		_ = hits
	}
}

var re2 = regexp.MustCompile("(" + strings.Join(dictionary2, "|") + ")")

func BenchmarkRegexpFails(b *testing.B) {
	for b.Loop() {
		re2.FindAllIndex(bytes, -1)
	}
}

var bytes2 = []byte("Firefox is a web browser, and is Mozilla's flagship software product. It is available in both desktop and mobile versions. Firefox uses the Gecko layout engine to render web pages, which implements current and anticipated web standards. As of April 2013, Firefox has approximately 20% of worldwide usage share of web browsers, making it the third most-used web browser. Firefox began as an experimental branch of the Mozilla codebase by Dave Hyatt, Joe Hewitt and Blake Ross. They believed the commercial requirements of Netscape's sponsorship and developer-driven feature creep compromised the utility of the Mozilla browser. To combat what they saw as the Mozilla Suite's software bloat, they created a stand-alone browser, with which they intended to replace the Mozilla Suite. Firefox was originally named Phoenix but the name was changed so as to avoid trademark conflicts with Phoenix Technologies. The initially-announced replacement, Firebird, provoked objections from the Firebird project community. The current name, Firefox, was chosen on February 9, 2004.")
var sbytes2 = string(bytes2)

var dictionary3 = []string{"Mozilla", "Mac", "Macintosh", "Safari", "Phoenix"}
var precomputed3 = NewStringMatcher(dictionary3)

func BenchmarkLongMatchWorks(b *testing.B) {
	for b.Loop() {
		precomputed3.Match(bytes2)
	}
}
func BenchmarkLongMatchThreadSafeWorks(b *testing.B) {
	for b.Loop() {
		precomputed3.MatchThreadSafe(bytes2)
	}
}

func BenchmarkLongContainsWorks(b *testing.B) {
	for b.Loop() {
		precomputed3.Contains(bytes2)
	}
}

func BenchmarkLongStringsContainsWorks(b *testing.B) {
	for b.Loop() {
		hits := make([]int, 0)
		for i, s := range dictionary3 {
			if strings.Contains(sbytes2, s) {
				hits = append(hits, i)
			}
		}
		_ = hits
	}
}

var re3 = regexp.MustCompile("(" + strings.Join(dictionary3, "|") + ")")

func BenchmarkLongRegexpWorks(b *testing.B) {
	for b.Loop() {
		re3.FindAllIndex(bytes2, -1)
	}
}

var dictionary4 = []string{"12343453", "34353", "234234523", "324234", "33333"}
var precomputed4 = NewStringMatcher(dictionary4)

func BenchmarkLongMatchFails(b *testing.B) {
	for b.Loop() {
		precomputed4.Match(bytes2)
	}
}

func BenchmarkLongContainsFails(b *testing.B) {
	for b.Loop() {
		precomputed4.Contains(bytes2)
	}
}

func BenchmarkLongStringsContainsFails(b *testing.B) {
	for b.Loop() {
		hits := make([]int, 0)
		for i, s := range dictionary4 {
			if strings.Contains(sbytes2, s) {
				hits = append(hits, i)
			}
		}
		_ = hits
	}
}

var re4 = regexp.MustCompile("(" + strings.Join(dictionary4, "|") + ")")

func BenchmarkLongRegexpFails(b *testing.B) {
	for b.Loop() {
		re4.FindAllIndex(bytes2, -1)
	}
}

var dictionary5 = []string{"12343453", "34353", "234234523", "324234", "33333", "experimental", "branch", "of", "the", "Mozilla", "codebase", "by", "Dave", "Hyatt", "Joe", "Hewitt", "and", "Blake", "Ross", "mother", "frequently", "performed", "in", "concerts", "around", "the", "village", "uses", "the", "Gecko", "layout", "engine"}
var precomputed5 = NewStringMatcher(dictionary5)

func BenchmarkMatchMany(b *testing.B) {
	for b.Loop() {
		precomputed5.Match(bytes)
	}
}

func BenchmarkMatchThreadSafeMany(b *testing.B) {
	for b.Loop() {
		precomputed5.MatchThreadSafe(bytes)
	}
}

func BenchmarkContainsMany(b *testing.B) {
	for b.Loop() {
		precomputed5.Contains(bytes)
	}
}

func BenchmarkStringsContainsMany(b *testing.B) {
	for b.Loop() {
		hits := make([]int, 0)
		for i, s := range dictionary5 {
			if strings.Contains(sbytes, s) {
				hits = append(hits, i)
			}
		}
		_ = hits
	}
}

var re5 = regexp.MustCompile("(" + strings.Join(dictionary5, "|") + ")")

func BenchmarkRegexpMany(b *testing.B) {
	for b.Loop() {
		re5.FindAllIndex(bytes, -1)
	}
}

func BenchmarkLongMatchMany(b *testing.B) {
	for b.Loop() {
		precomputed5.Match(bytes2)
	}
}

func BenchmarkLongMatchThreadSafeMany(b *testing.B) {
	for b.Loop() {
		precomputed5.MatchThreadSafe(bytes2)
	}
}

func BenchmarkLongContainsMany(b *testing.B) {
	for b.Loop() {
		precomputed5.Contains(bytes2)
	}
}

func BenchmarkLongStringsContainsMany(b *testing.B) {
	for b.Loop() {
		hits := make([]int, 0)
		for i, s := range dictionary5 {
			if strings.Contains(sbytes2, s) {
				hits = append(hits, i)
			}
		}
		_ = hits
	}
}

func BenchmarkLongRegexpMany(b *testing.B) {
	for b.Loop() {
		re5.FindAllIndex(bytes2, -1)
	}
}

var dictionary6 = []string{"2004", "2013", "9", "a", "an", "and", "anticipated", "approximately", "April", "as", "available", "avoid", "began", "believed", "Blake", "bloat", "both", "branch", "browser", "browsers", "but", "by", "changed", "chosen", "codebase", "combat", "commercial", "community", "compromised", "conflicts", "created", "creep", "current", "Dave", "desktop", "developer-driven", "engine", "experimental", "feature", "February", "Firebird", "Firefox", "flagship", "from", "Gecko", "has", "Hewitt", "Hyatt", "implements", "in", "initially-announced", "intended", "is", "it", "Joe", "layout", "making", "mobile", "most-used", "Mozilla", "Mozilla's", "name", "named", "Netscape's", "objections", "of", "on", "originally", "pages", "Phoenix", "product", "project", "provoked", "render", "replace", "replacement", "requirements", "Ross", "saw", "share", "so", "software", "sponsorship", "stand-alone", "standards", "Suite", "Suite's", "Technologies", "the", "The", "they", "They", "third", "to", "trademark", "usage", "uses", "utility", "versions", "was", "web", "what", "which", "with", "worldwide"}
var precomputed6 = NewStringMatcher(dictionary6)

func BenchmarkLargeMatchWorks(b *testing.B) {
	for b.Loop() {
		precomputed6.Match(bytes2)
	}
}

func BenchmarkLargeMatchThreadSafeWorks(b *testing.B) {
	for b.Loop() {
		precomputed6.MatchThreadSafe(bytes2)
	}
}

func BenchmarkLargeContainsWorks(b *testing.B) {
	for b.Loop() {
		precomputed6.Contains(bytes2)
	}
}

func BenchmarkLargeContainsFails(b *testing.B) {
	m := NewStringMatcher([]string{"zzz", "qqq", "xxx"})
	for b.Loop() {
		m.Contains(bytes2)
	}
}

// BenchmarkNewStringMatcher measures trie construction time for a small dictionary.
func BenchmarkNewStringMatcher(b *testing.B) {
	for b.Loop() {
		NewStringMatcher(dictionary)
	}
}

// BenchmarkNewStringMatcherLarge measures trie construction time for a large dictionary.
func BenchmarkNewStringMatcherLarge(b *testing.B) {
	for b.Loop() {
		NewStringMatcher(dictionary6)
	}
}

// BenchmarkMatchThreadSafeConcurrent measures MatchThreadSafe under real parallel load.
func BenchmarkMatchThreadSafeConcurrent(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			precomputed6.MatchThreadSafe(bytes2)
		}
	})
}
