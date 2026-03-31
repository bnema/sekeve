package fuzzysearch

import "testing"

func TestScore_ExactMatch(t *testing.T) {
	s := Score("github", "GitHub")
	if s <= 0 {
		t.Errorf("exact match should score > 0, got %d", s)
	}
}

func TestScore_PrefixMatch(t *testing.T) {
	s := Score("git", "GitHub")
	if s <= 0 {
		t.Errorf("prefix match should score > 0, got %d", s)
	}
}

func TestScore_NoMatch(t *testing.T) {
	s := Score("xyz", "GitHub")
	if s != 0 {
		t.Errorf("no match should score 0, got %d", s)
	}
}

func TestScore_WordBoundaryBonus(t *testing.T) {
	boundary := Score("sc", "SSH Config")
	interior := Score("sc", "obscure")
	if boundary <= interior {
		t.Errorf("word boundary (%d) should outscore interior (%d)", boundary, interior)
	}
}

func TestScore_CaseInsensitive(t *testing.T) {
	upper := Score("GIT", "github")
	lower := Score("git", "GitHub")
	if upper != lower {
		t.Errorf("case insensitive: %d != %d", upper, lower)
	}
}

func TestSearch_ReturnsTopN(t *testing.T) {
	items := []string{"GitHub", "GitLab", "Bitbucket", "Azure DevOps", "Gitea"}
	results := Search("git", items, 3)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	for _, m := range results {
		if m.Score <= 0 {
			t.Errorf("matched item should have positive score, got %d", m.Score)
		}
	}
}

func TestSearch_NoResults(t *testing.T) {
	items := []string{"GitHub", "GitLab"}
	results := Search("zzz", items, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestScore_UnicodeMatching(t *testing.T) {
	s := Score("café", "café au lait")
	if s <= 0 {
		t.Errorf("unicode prefix match should score > 0, got %d", s)
	}

	s = Score("naïve", "naïve approach")
	if s <= 0 {
		t.Errorf("unicode match with diaeresis should score > 0, got %d", s)
	}

	s = Score("日本", "日本語テスト")
	if s <= 0 {
		t.Errorf("CJK match should score > 0, got %d", s)
	}
}

func TestSearch_MultiWord(t *testing.T) {
	items := []string{"Git SSH Config", "GitHub Actions", "SSH Key"}
	results := Search("git ssh", items, 5)
	if len(results) == 0 {
		t.Fatal("multi-word search should return results")
	}
	// "Git SSH Config" should rank first (matches both words)
	if results[0].Index != 0 {
		t.Errorf("expected 'Git SSH Config' first, got index %d", results[0].Index)
	}
}
