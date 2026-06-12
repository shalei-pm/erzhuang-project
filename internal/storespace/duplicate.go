package storespace

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
	for _, suffix := range []string{"新氧青春", "门店", "店", "分院", "院"} {
		normalized = strings.ReplaceAll(normalized, suffix, "")
	}
	return normalized
}

func IsSimilarStoreName(left, right string) bool {
	left = NormalizeStoreName(left)
	right = NormalizeStoreName(right)
	if left == "" || right == "" || left == right {
		return left == right
	}
	return strings.Contains(left, right) || strings.Contains(right, left)
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
