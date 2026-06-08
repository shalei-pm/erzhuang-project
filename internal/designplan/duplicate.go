package designplan

import (
	"strings"
	"unicode"
)

func NormalizeStoreName(name string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(strings.ToLower(name)) {
		if unicode.IsSpace(r) || r == '-' || r == '_' || r == '·' || r == '・' {
			continue
		}
		if r == '（' || r == '(' || r == '）' || r == ')' {
			continue
		}
		builder.WriteRune(r)
	}

	normalized := builder.String()
	replacers := []struct {
		old string
		new string
	}{
		{"新氧青春", ""},
		{"门店", ""},
		{"店", ""},
		{"分院", ""},
		{"院", ""},
	}
	for _, replacer := range replacers {
		normalized = strings.ReplaceAll(normalized, replacer.old, replacer.new)
	}
	return normalized
}

func IsSimilarStoreName(left, right string) bool {
	left = NormalizeStoreName(left)
	right = NormalizeStoreName(right)
	if left == "" || right == "" || left == right {
		return left == right
	}
	if strings.Contains(left, right) || strings.Contains(right, left) {
		return true
	}
	return levenshteinDistance(left, right) <= 2
}

func MatchesStoreSearch(name string, normalizedName string, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}
	rawName := strings.ToLower(strings.ReplaceAll(name, " ", ""))
	rawQuery := strings.ToLower(strings.ReplaceAll(query, " ", ""))
	if rawQuery != "" && strings.Contains(rawName, rawQuery) {
		return true
	}
	normalizedQuery := NormalizeStoreName(query)
	return normalizedQuery != "" && strings.Contains(normalizedName, normalizedQuery)
}

func levenshteinDistance(left, right string) int {
	l := []rune(left)
	r := []rune(right)
	if len(l) == 0 {
		return len(r)
	}
	if len(r) == 0 {
		return len(l)
	}

	previous := make([]int, len(r)+1)
	current := make([]int, len(r)+1)
	for i := range previous {
		previous[i] = i
	}

	for i := 1; i <= len(l); i++ {
		current[0] = i
		for j := 1; j <= len(r); j++ {
			cost := 0
			if l[i-1] != r[j-1] {
				cost = 1
			}
			current[j] = minInt(
				previous[j]+1,
				current[j-1]+1,
				previous[j-1]+cost,
			)
		}
		copy(previous, current)
	}
	return previous[len(r)]
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}
