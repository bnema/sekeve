package focusring

// Focusable is the interface a widget must implement to participate in a focus ring.
type Focusable interface {
	GrabFocus()
	HasFocus() bool
	SetVisible(bool)
	GetVisible() bool
}

// Ring cycles focus through an ordered list of widgets.
type Ring struct {
	widgets []Focusable
	current int // -1 = no focus yet, 0..len-1 = focused index
}

// New creates a Ring with the given widgets. Focus starts before the first widget.
func New(widgets ...Focusable) *Ring {
	return &Ring{widgets: widgets, current: -1}
}

// Next moves focus to the next widget and returns it.
// Wraps around to the first widget after the last.
func (r *Ring) Next() Focusable {
	if len(r.widgets) == 0 {
		return nil
	}
	r.current = (r.current + 1) % len(r.widgets)
	w := r.widgets[r.current]
	w.GrabFocus()
	return w
}

// Prev moves focus to the previous widget and returns it.
// Wraps around to the last widget before the first.
func (r *Ring) Prev() Focusable {
	if len(r.widgets) == 0 {
		return nil
	}
	r.current--
	if r.current < 0 {
		r.current = len(r.widgets) - 1
	}
	w := r.widgets[r.current]
	w.GrabFocus()
	return w
}

// Focus sets the ring's current position to the given widget.
// If w is not in the ring, this is a no-op.
func (r *Ring) Focus(w Focusable) {
	for i, widget := range r.widgets {
		if widget == w {
			r.current = i
			w.GrabFocus()
			return
		}
	}
}

// Reset moves the cursor back to before the first widget.
func (r *Ring) Reset() {
	r.current = -1
}

// SetWidgets replaces the widget list and resets the cursor.
func (r *Ring) SetWidgets(widgets ...Focusable) {
	r.widgets = widgets
	r.current = -1
}

// Current returns the currently focused widget, or nil.
func (r *Ring) Current() Focusable {
	if r.current < 0 || r.current >= len(r.widgets) {
		return nil
	}
	return r.widgets[r.current]
}
