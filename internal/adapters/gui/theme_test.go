//go:build gtk && linux && !nogtk

package gui

import (
	"strings"
	"testing"
)

func TestEmeraldCSSUsesWarmAmberForInteractiveStates(t *testing.T) {
	checks := map[string]string{
		"active tab background":   "#c9891a",
		"focus ring border":       "#f59e0b",
		"selected row background": "rgba(245, 158, 11, 0.08)",
		"save button background":  "#d97706",
	}

	for name, want := range checks {
		if !strings.Contains(emeraldCSS, want) {
			t.Fatalf("%s: expected theme CSS to contain %q", name, want)
		}
	}
}
