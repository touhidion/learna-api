package utils

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)
	dashRuns     = regexp.MustCompile(`-{2,}`)
)

// maxSlugLen keeps slugs comfortably inside URL and index limits while leaving
// room for a disambiguating suffix.
const maxSlugLen = 80

// Slugify converts a title into a URL-safe slug: accents folded to ASCII,
// lowercased, non-alphanumerics collapsed to single hyphens.
//
//	"Introducción to Go!!  " -> "introduccion-to-go"
//
// A title with no usable characters yields "", and callers should fall back to
// SlugWithSuffix to produce something unique.
func Slugify(title string) string {
	// Decompose accents (NFD), drop the combining marks, recompose.
	folded, _, err := transform.String(
		transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC),
		title,
	)
	if err != nil {
		folded = title
	}

	s := strings.ToLower(strings.TrimSpace(folded))
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = dashRuns.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if len(s) > maxSlugLen {
		s = strings.Trim(s[:maxSlugLen], "-")
	}
	return s
}

// SlugWithSuffix appends a numeric suffix, for resolving collisions:
//
//	SlugWithSuffix("intro-to-go", 2) -> "intro-to-go-2"
//
// The base is truncated if needed so the result still fits maxSlugLen.
func SlugWithSuffix(base string, n int) string {
	suffix := fmt.Sprintf("-%d", n)
	if len(base)+len(suffix) > maxSlugLen {
		base = strings.Trim(base[:maxSlugLen-len(suffix)], "-")
	}
	return base + suffix
}
