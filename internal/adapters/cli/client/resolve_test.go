package client

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPickerModelFromResult_WrongType(t *testing.T) {
	_, err := pickerModelFromResult(testModel{})
	if err == nil {
		t.Fatal("expected error for unexpected picker result type")
	}
	if !strings.Contains(err.Error(), "unexpected picker result type") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "unexpected picker result type")
	}
}

func TestPickerModelFromResult_Success(t *testing.T) {
	want := pickerModel{selected: 1}

	got, err := pickerModelFromResult(want)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.selected != want.selected {
		t.Fatalf("selected = %d, want %d", got.selected, want.selected)
	}
}

type testModel struct{}

func (testModel) Init() tea.Cmd { return nil }

func (m testModel) Update(_ tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (testModel) View() tea.View { return tea.NewView("") }
