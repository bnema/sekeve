package focusring

import "testing"

// mockWidget implements Focusable for testing.
type mockWidget struct {
	focused bool
	name    string
}

func (m *mockWidget) GrabFocus()       { m.focused = true }
func (m *mockWidget) HasFocus() bool   { return m.focused }
func (m *mockWidget) SetVisible(bool)  {}
func (m *mockWidget) GetVisible() bool { return true }

func newMock(name string) *mockWidget { return &mockWidget{name: name} }

func TestRing_Next_CyclesForward(t *testing.T) {
	a, b, c := newMock("a"), newMock("b"), newMock("c")
	r := New(a, b, c)

	got := r.Next()
	if got != a {
		t.Errorf("first Next should return first widget")
	}
	got = r.Next()
	if got != b {
		t.Errorf("second Next should return second widget")
	}
	got = r.Next()
	if got != c {
		t.Errorf("third Next should return third widget")
	}
	got = r.Next()
	if got != a {
		t.Errorf("fourth Next should wrap to first widget")
	}
}

func TestRing_Prev_CyclesBackward(t *testing.T) {
	a, b, c := newMock("a"), newMock("b"), newMock("c")
	r := New(a, b, c)

	got := r.Prev()
	if got != c {
		t.Errorf("first Prev should return last widget, got %s", got.(*mockWidget).name)
	}
	got = r.Prev()
	if got != b {
		t.Errorf("second Prev should return second-to-last")
	}
}

func TestRing_Empty(t *testing.T) {
	r := New()
	if r.Next() != nil {
		t.Error("Next on empty ring should return nil")
	}
	if r.Prev() != nil {
		t.Error("Prev on empty ring should return nil")
	}
}

func TestRing_Reset(t *testing.T) {
	a, b := newMock("a"), newMock("b")
	r := New(a, b)
	r.Next() // a
	r.Next() // b
	r.Reset()
	got := r.Next()
	if got != a {
		t.Error("after Reset, Next should return first widget")
	}
}

func TestRing_SetWidgets(t *testing.T) {
	a, b := newMock("a"), newMock("b")
	r := New(a)
	r.Next() // a
	c := newMock("c")
	r.SetWidgets(b, c)
	got := r.Next()
	if got != b {
		t.Error("after SetWidgets, Next should return new first widget")
	}
}
