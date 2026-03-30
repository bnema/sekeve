package gtkutil

import "testing"

func TestRetainCallback_AppendsToSlice(t *testing.T) {
	var callbacks []interface{}
	cb := func() {}
	RetainCallback(&callbacks, cb)
	if len(callbacks) != 1 {
		t.Errorf("expected 1 callback, got %d", len(callbacks))
	}
	RetainCallback(&callbacks, cb)
	if len(callbacks) != 2 {
		t.Errorf("expected 2 callbacks, got %d", len(callbacks))
	}
}
