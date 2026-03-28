//go:build linux

package gui

import (
	"image"
	"image/color"
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var (
	bgColor       = color.NRGBA{R: 0x1B, G: 0x1B, B: 0x1F, A: 0xFF}
	borderNormal  = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
	borderError   = color.NRGBA{R: 0xE5, G: 0x48, B: 0x4D, A: 0xFF}
	textColor     = color.NRGBA{R: 0xFA, G: 0xFA, B: 0xFA, A: 0xFF}
	msgColor      = color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xFF}
	msgErrorColor = color.NRGBA{R: 0xE5, G: 0x48, B: 0x4D, A: 0xFF}
	inputBg       = color.NRGBA{R: 0x25, G: 0x25, B: 0x2A, A: 0xFF}
	inputBgError  = color.NRGBA{R: 0x2A, G: 0x20, B: 0x20, A: 0xFF}
)

// RunPINPrompt opens a Gio window for PIN entry.
// Returns the entered PIN string, or empty string if cancelled.
// errorMode enables red styling. message is optional text above the input.
func RunPINPrompt(errorMode bool, message string) (string, error) {
	var pin string

	go func() {
		var w app.Window

		winHeight := unit.Dp(80)
		if message != "" {
			winHeight = unit.Dp(120)
		}
		w.Option(app.Title("Sekeve"), app.Size(unit.Dp(350), winHeight))

		th := material.NewTheme()
		th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

		var (
			ops    op.Ops
			editor widget.Editor
		)
		editor.SingleLine = true
		editor.Submit = true
		editor.Mask = '●'

		focused := false

		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				return

			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)

				// Auto-focus editor on first frame.
				if !focused {
					gtx.Execute(key.FocusCmd{Tag: &editor})
					focused = true
				}

				// Handle Escape key to cancel.
				if _, ok := gtx.Event(key.Filter{Name: key.NameEscape}); ok {
					w.Perform(system.ActionClose)
					os.Exit(1)
				}

				// Process editor events (Submit on Enter).
				for {
					ev, ok := editor.Update(gtx)
					if !ok {
						break
					}
					if _, isSubmit := ev.(widget.SubmitEvent); isSubmit {
						pin = editor.Text()
						w.Perform(system.ActionClose)
					}
				}

				// Draw background.
				paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())

				// Layout content.
				layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						// Optional message above input.
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if message == "" {
								return layout.Dimensions{}
							}
							lbl := material.Body2(th, message)
							lbl.Color = msgColor
							if errorMode {
								lbl.Color = msgErrorColor
							}
							dims := lbl.Layout(gtx)
							// Add spacing below message.
							dims.Size.Y += gtx.Dp(unit.Dp(8))
							return dims
						}),
						// Bordered input field.
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return drawBorderedInput(gtx, th, &editor, errorMode)
						}),
					)
				})

				e.Frame(gtx.Ops)
			}
		}
	}()

	app.Main()
	return pin, nil
}

// drawBorderedInput draws a rounded border around the editor input field.
func drawBorderedInput(gtx layout.Context, th *material.Theme, editor *widget.Editor, errorMode bool) layout.Dimensions {
	borderColor := borderNormal
	bgColor := inputBg
	if errorMode {
		borderColor = borderError
		bgColor = inputBgError
	}

	// Measure the inner content first to get desired height.
	innerInset := layout.UniformInset(unit.Dp(8))

	// Calculate dimensions with a fixed minimum height for the input box.
	minHeight := gtx.Dp(unit.Dp(36))
	gtx.Constraints.Min.Y = minHeight
	if gtx.Constraints.Max.Y < minHeight {
		gtx.Constraints.Max.Y = minHeight
	}

	// Draw border (outer rounded rect).
	borderWidth := 1
	totalWidth := gtx.Constraints.Max.X
	totalHeight := minHeight

	outerRect := image.Rectangle{Max: image.Pt(totalWidth, totalHeight)}
	innerRect := image.Rectangle{
		Min: image.Pt(borderWidth, borderWidth),
		Max: image.Pt(totalWidth-borderWidth, totalHeight-borderWidth),
	}

	// Paint border.
	paint.FillShape(gtx.Ops, borderColor,
		clip.RRect{Rect: outerRect, SE: 4, SW: 4, NE: 4, NW: 4}.Op(gtx.Ops))

	// Paint inner background.
	paint.FillShape(gtx.Ops, bgColor,
		clip.RRect{Rect: innerRect, SE: 3, SW: 3, NE: 3, NW: 3}.Op(gtx.Ops))

	// Clip to inner area and draw editor.
	innerStack := clip.RRect{Rect: innerRect, SE: 3, SW: 3, NE: 3, NW: 3}.Push(gtx.Ops)
	innerGtx := gtx
	innerGtx.Constraints.Max.X = innerRect.Dx()
	innerGtx.Constraints.Max.Y = innerRect.Dy()
	innerGtx.Constraints.Min.X = innerRect.Dx()
	innerGtx.Constraints.Min.Y = 0

	editorStyle := material.Editor(th, editor, "")
	editorStyle.Color = textColor
	innerInset.Layout(innerGtx, editorStyle.Layout)
	innerStack.Pop()

	return layout.Dimensions{Size: image.Pt(totalWidth, totalHeight)}
}
