//go:build !goexperiment.simd

package search

import "strings"

const Backend = "standard"

func Contains(text, query string) bool { return strings.Contains(text, query) }
