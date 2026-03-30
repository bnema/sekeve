package fuzzysearch

import (
	"sort"
	"strings"
	"unicode"
)

// Match represents a search result with its original index and score.
type Match struct {
	Index int
	Score int
}

// Score computes a fuzzy match score for query against target.
// Returns 0 if query doesn't match. Higher is better.
// Case-insensitive. Awards bonuses for:
//   - word boundary matches (+10)
//   - consecutive character matches (+5 each)
//   - prefix matches (+20)
func Score(query, target string) int {
	q := strings.ToLower(query)
	t := strings.ToLower(target)

	if q == "" {
		return 0
	}
	if q == t {
		return 1000
	}

	score := 0
	qi := 0
	consecutive := 0

	for ti := 0; ti < len(t) && qi < len(q); ti++ {
		atBoundary := ti == 0
		if ti > 0 {
			prev := rune(t[ti-1])
			atBoundary = !unicode.IsLetter(prev) && !unicode.IsDigit(prev)
		}

		if t[ti] == q[qi] {
			score++
			if atBoundary {
				score += 10
			}
			if qi == 0 && ti == 0 {
				score += 20
			}
			consecutive++
			if consecutive > 1 {
				score += 5
			}
			qi++
		} else {
			consecutive = 0
		}
	}

	if qi < len(q) {
		return 0
	}
	return score
}

// Search finds the top `limit` items matching query, sorted by score descending.
// Supports multi-word queries: each word must match independently, scores are summed.
// Items with zero score are excluded.
func Search(query string, items []string, limit int) []Match {
	words := strings.Fields(query)
	if len(words) == 0 {
		return nil
	}

	var matches []Match
	for i, item := range items {
		totalScore := 0
		allMatch := true
		for _, word := range words {
			s := Score(word, item)
			if s == 0 {
				allMatch = false
				break
			}
			totalScore += s
		}
		if allMatch && totalScore > 0 {
			matches = append(matches, Match{Index: i, Score: totalScore})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}
