//go:build goexperiment.simd

package search

import (
	"simd"
	"strings"
)

const Backend = "simd"

// Contains rejects vectors without the first query byte, then checks candidate spans.
// The scalar tail also handles short strings and arbitrary UTF-8 byte sequences.
func Contains(text, query string) bool {
	if len(query) == 0 {
		return true
	}
	needle := simd.BroadcastUint8s(query[0])
	width := needle.Len()
	matches := make([]int8, width)
	for len(text) >= width+len(query)-1 {
		block := []byte(text[:width])
		simd.LoadUint8s(block).Equal(needle).ToInt8s().Store(matches)
		for i, match := range matches {
			if match != 0 && strings.HasPrefix(text[i:], query) {
				return true
			}
		}
		text = text[width:]
	}
	return strings.Contains(text, query)
}
