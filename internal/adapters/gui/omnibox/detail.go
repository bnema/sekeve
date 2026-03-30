// internal/adapters/gui/omnibox/detail.go
//go:build linux

package omnibox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/sekeve/internal/adapters/gui/widget"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/sekeve/pkg/focusring"
	"github.com/bnema/sekeve/pkg/gtkutil"
)

// detailFormKind identifies which detail form is displayed.
type detailFormKind int

const (
	detailFormLogin detailFormKind = iota
	detailFormNote
	detailFormSecret
)

// DetailView displays and edits a single entry with per-field copy buttons.
type DetailView struct {
	Root *gtk.Box

	cfg    port.OmniboxConfig
	ctx    context.Context
	kind   detailFormKind
	env    *entity.Envelope // the envelope being edited
	onDone func()           // called after save or back to return to search

	// Login form fields.
	loginSite     *widget.LabeledEntry
	loginSiteCopy *gtk.Button
	loginUsername *widget.LabeledEntry
	loginUserCopy *gtk.Button
	loginPassword *widget.LabeledPassword
	loginPassCopy *gtk.Button
	loginNotes    *widget.LabeledEntry
	loginNoteCopy *gtk.Button

	// Note form fields.
	noteName     *widget.LabeledEntry
	noteNameCopy *gtk.Button
	noteContent  *gtk.TextView
	noteContCopy *gtk.Button

	// Secret form fields.
	secretName     *widget.LabeledEntry
	secretNameCopy *gtk.Button
	secretValue    *widget.LabeledPassword
	secretValCopy  *gtk.Button

	backBtn *gtk.Button
	saveBtn *gtk.Button

	callbacks []interface{}
}

// NewDetailView creates the detail view container.
func NewDetailView(ctx context.Context, cfg port.OmniboxConfig, onDone func()) *DetailView {
	root, _ := gtkutil.SafeNewWidget("detail-root", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationVerticalValue, 6)
	})

	dv := &DetailView{
		Root:   root,
		cfg:    cfg,
		ctx:    ctx,
		onDone: onDone,
	}

	if root != nil {
		root.SetMarginStart(12)
		root.SetMarginEnd(12)
		root.SetMarginTop(8)
		root.SetMarginBottom(8)
		root.SetVisible(false)
	}

	return dv
}

// Show populates the detail view with the given envelope and decrypted payload.
func (dv *DetailView) Show(env *entity.Envelope, plaintext []byte) {
	dv.env = env
	switch env.Type {
	case entity.EntryTypeNote:
		dv.kind = detailFormNote
	case entity.EntryTypeSecret:
		dv.kind = detailFormSecret
	default:
		dv.kind = detailFormLogin
	}
	dv.buildForm(plaintext)
	if dv.Root != nil {
		dv.Root.SetVisible(true)
	}
}

// Hide hides the detail view.
func (dv *DetailView) Hide() {
	if dv.Root != nil {
		dv.Root.SetVisible(false)
	}
}

// Focusables returns the form fields, copy buttons, and action buttons
// for focus ring integration.
func (dv *DetailView) Focusables() []focusring.Focusable {
	var out []focusring.Focusable
	switch dv.kind {
	case detailFormLogin:
		out = appendFieldAndCopy(out, entryWidget(dv.loginSite), dv.loginSiteCopy)
		out = appendFieldAndCopy(out, entryWidget(dv.loginUsername), dv.loginUserCopy)
		out = appendFieldAndCopy(out, passwordWidget(dv.loginPassword), dv.loginPassCopy)
		out = appendFieldAndCopy(out, entryWidget(dv.loginNotes), dv.loginNoteCopy)
	case detailFormNote:
		out = appendFieldAndCopy(out, entryWidget(dv.noteName), dv.noteNameCopy)
		if dv.noteContent != nil {
			out = append(out, &focusableWidget{&dv.noteContent.Widget})
		}
		out = appendCopyBtn(out, dv.noteContCopy)
	case detailFormSecret:
		out = appendFieldAndCopy(out, entryWidget(dv.secretName), dv.secretNameCopy)
		out = appendFieldAndCopy(out, passwordWidget(dv.secretValue), dv.secretValCopy)
	}
	if dv.backBtn != nil {
		out = append(out, &focusableWidget{&dv.backBtn.Widget})
	}
	if dv.saveBtn != nil {
		out = append(out, &focusableWidget{&dv.saveBtn.Widget})
	}
	return out
}

// buildForm clears the root box and populates it with fields for the entry.
func (dv *DetailView) buildForm(plaintext []byte) {
	if dv.Root == nil {
		return
	}
	dv.clearRoot()
	dv.resetFields()

	switch dv.kind {
	case detailFormLogin:
		dv.buildLoginDetail(plaintext)
	case detailFormNote:
		dv.buildNoteDetail(plaintext)
	case detailFormSecret:
		dv.buildSecretDetail(plaintext)
	}

	dv.buildButtons()
}

func (dv *DetailView) buildLoginDetail(plaintext []byte) {
	var login entity.Login
	_ = json.Unmarshal(plaintext, &login)

	dv.loginSite = widget.NewLabeledEntry("Site", "https://example.com")
	if dv.loginSite.Entry != nil {
		dv.loginSite.Entry.SetText(login.Site)
	}
	dv.loginSiteCopy = dv.newCopyButton(func() string {
		if dv.loginSite != nil && dv.loginSite.Entry != nil {
			return dv.loginSite.Entry.GetText()
		}
		return ""
	})
	dv.appendFieldRow(dv.loginSite.Box, dv.loginSiteCopy)

	dv.loginUsername = widget.NewLabeledEntry("Username", "user@example.com")
	if dv.loginUsername.Entry != nil {
		dv.loginUsername.Entry.SetText(login.Username)
	}
	dv.loginUserCopy = dv.newCopyButton(func() string {
		if dv.loginUsername != nil && dv.loginUsername.Entry != nil {
			return dv.loginUsername.Entry.GetText()
		}
		return ""
	})
	dv.appendFieldRow(dv.loginUsername.Box, dv.loginUserCopy)

	dv.loginPassword = widget.NewLabeledPassword("Password", "")
	if dv.loginPassword.Entry != nil {
		dv.loginPassword.Entry.SetText(login.Password)
	}
	dv.loginPassCopy = dv.newCopyButton(func() string {
		if dv.loginPassword != nil && dv.loginPassword.Entry != nil {
			return dv.loginPassword.Entry.GetText()
		}
		return ""
	})
	dv.appendFieldRow(dv.loginPassword.Box, dv.loginPassCopy)

	dv.loginNotes = widget.NewLabeledEntry("Notes", "")
	if dv.loginNotes.Entry != nil {
		dv.loginNotes.Entry.SetText(login.Notes)
	}
	dv.loginNoteCopy = dv.newCopyButton(func() string {
		if dv.loginNotes != nil && dv.loginNotes.Entry != nil {
			return dv.loginNotes.Entry.GetText()
		}
		return ""
	})
	dv.appendFieldRow(dv.loginNotes.Box, dv.loginNoteCopy)
}

func (dv *DetailView) buildNoteDetail(plaintext []byte) {
	var note entity.Note
	_ = json.Unmarshal(plaintext, &note)

	dv.noteName = widget.NewLabeledEntry("Name", "My note")
	if dv.noteName.Entry != nil {
		dv.noteName.Entry.SetText(note.Name)
	}
	dv.noteNameCopy = dv.newCopyButton(func() string {
		if dv.noteName != nil && dv.noteName.Entry != nil {
			return dv.noteName.Entry.GetText()
		}
		return ""
	})
	dv.appendFieldRow(dv.noteName.Box, dv.noteNameCopy)

	// Content label.
	contentLabel := "Content"
	label, _ := gtkutil.SafeNewWidget("detail-note-label", func() *gtk.Label {
		return gtk.NewLabel(&contentLabel)
	})
	if label != nil {
		label.AddCssClass("sekeve-label")
		label.SetHalign(gtk.AlignStartValue)
	}

	// Multiline text view in a scrolled window.
	dv.noteContent, _ = gtkutil.SafeNewWidget("detail-note-textview", func() *gtk.TextView {
		return gtk.NewTextView()
	})
	if dv.noteContent != nil {
		dv.noteContent.SetWrapMode(gtk.WrapWordValue)
		buf := dv.noteContent.GetBuffer()
		if buf != nil {
			buf.SetText(note.Content, len(note.Content))
		}
	}

	scroll, _ := gtkutil.SafeNewWidget("detail-note-scroll", func() *gtk.ScrolledWindow {
		return gtk.NewScrolledWindow()
	})

	dv.noteContCopy = dv.newCopyButton(func() string {
		if dv.noteContent != nil {
			return getTextViewContent(dv.noteContent)
		}
		return ""
	})

	// Build a row with label + content + copy button.
	fieldBox, _ := gtkutil.SafeNewWidget("detail-note-field", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationVerticalValue, 3)
	})
	if fieldBox != nil {
		if label != nil {
			fieldBox.Append(&label.Widget)
		}
		if scroll != nil {
			scroll.SetPolicy(gtk.PolicyNeverValue, gtk.PolicyAutomaticValue)
			scroll.SetMinContentHeight(80)
			scroll.SetMaxContentHeight(200)
			if dv.noteContent != nil {
				scroll.SetChild(&dv.noteContent.Widget)
			}
			fieldBox.Append(&scroll.Widget)
		}
	}

	dv.appendFieldRow(fieldBox, dv.noteContCopy)
}

func (dv *DetailView) buildSecretDetail(plaintext []byte) {
	var secret entity.Secret
	_ = json.Unmarshal(plaintext, &secret)

	dv.secretName = widget.NewLabeledEntry("Name", "API key name")
	if dv.secretName.Entry != nil {
		dv.secretName.Entry.SetText(secret.Name)
	}
	dv.secretNameCopy = dv.newCopyButton(func() string {
		if dv.secretName != nil && dv.secretName.Entry != nil {
			return dv.secretName.Entry.GetText()
		}
		return ""
	})
	dv.appendFieldRow(dv.secretName.Box, dv.secretNameCopy)

	dv.secretValue = widget.NewLabeledPassword("Value", "")
	if dv.secretValue.Entry != nil {
		dv.secretValue.Entry.SetText(secret.Value)
	}
	dv.secretValCopy = dv.newCopyButton(func() string {
		if dv.secretValue != nil && dv.secretValue.Entry != nil {
			return dv.secretValue.Entry.GetText()
		}
		return ""
	})
	dv.appendFieldRow(dv.secretValue.Box, dv.secretValCopy)
}

func (dv *DetailView) buildButtons() {
	btnBox, _ := gtkutil.SafeNewWidget("detail-btn-box", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationHorizontalValue, 8)
	})
	if btnBox == nil {
		return
	}
	btnBox.SetMarginTop(8)

	backLabel := "\u2190 Back"
	dv.backBtn, _ = gtkutil.SafeNewWidget("detail-back", func() *gtk.Button {
		return gtk.NewButtonWithLabel(backLabel)
	})
	if dv.backBtn != nil {
		dv.backBtn.AddCssClass("sekeve-btn-cancel")
		dv.backBtn.SetHexpand(true)
		dv.backBtn.SetHalign(gtk.AlignStartValue)
		backCb := func(_ gtk.Button) {
			dv.doBack()
		}
		gtkutil.RetainCallback(&dv.callbacks, backCb)
		dv.backBtn.ConnectClicked(&backCb)
		btnBox.Append(&dv.backBtn.Widget)
	}

	dv.saveBtn, _ = gtkutil.SafeNewWidget("detail-save", func() *gtk.Button {
		return gtk.NewButtonWithLabel("Save")
	})
	if dv.saveBtn != nil {
		dv.saveBtn.AddCssClass("sekeve-btn-save")
		dv.saveBtn.SetHalign(gtk.AlignEndValue)
		saveCb := func(_ gtk.Button) {
			dv.doSave()
		}
		gtkutil.RetainCallback(&dv.callbacks, saveCb)
		dv.saveBtn.ConnectClicked(&saveCb)
		btnBox.Append(&dv.saveBtn.Widget)
	}

	dv.Root.Append(&btnBox.Widget)
}

// newCopyButton creates a copy button that copies the value returned by getValue.
func (dv *DetailView) newCopyButton(getValue func() string) *gtk.Button {
	copyLabel := "\u29C9" // ⧉
	btn, _ := gtkutil.SafeNewWidget("detail-copy-btn", func() *gtk.Button {
		return gtk.NewButtonWithLabel(copyLabel)
	})
	if btn != nil {
		btn.AddCssClass("sekeve-copy-btn")
		ctx := dv.ctx
		clickCb := func(_ gtk.Button) {
			value := getValue()
			if value != "" {
				go copyToClipboard(ctx, value)
			}
		}
		gtkutil.RetainCallback(&dv.callbacks, clickCb)
		btn.ConnectClicked(&clickCb)
	}
	return btn
}

// appendFieldRow appends a horizontal box containing the field box and copy button.
func (dv *DetailView) appendFieldRow(fieldBox *gtk.Box, copyBtn *gtk.Button) {
	if dv.Root == nil {
		return
	}
	row, _ := gtkutil.SafeNewWidget("detail-field-row", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationHorizontalValue, 4)
	})
	if row == nil {
		return
	}

	if fieldBox != nil {
		fieldBox.SetHexpand(true)
		row.Append(&fieldBox.Widget)
	}
	if copyBtn != nil {
		copyBtn.SetValign(gtk.AlignEndValue)
		row.Append(&copyBtn.Widget)
	}

	dv.Root.Append(&row.Widget)
}

// clearRoot removes all children from the root box.
func (dv *DetailView) clearRoot() {
	if dv.Root == nil {
		return
	}
	for {
		child := dv.Root.GetFirstChild()
		if child == nil {
			break
		}
		dv.Root.Remove(child)
	}
}

// resetFields zeroes all field pointers.
func (dv *DetailView) resetFields() {
	dv.loginSite = nil
	dv.loginSiteCopy = nil
	dv.loginUsername = nil
	dv.loginUserCopy = nil
	dv.loginPassword = nil
	dv.loginPassCopy = nil
	dv.loginNotes = nil
	dv.loginNoteCopy = nil
	dv.noteName = nil
	dv.noteNameCopy = nil
	dv.noteContent = nil
	dv.noteContCopy = nil
	dv.secretName = nil
	dv.secretNameCopy = nil
	dv.secretValue = nil
	dv.secretValCopy = nil
	dv.backBtn = nil
	dv.saveBtn = nil
}

func (dv *DetailView) doBack() {
	dv.Hide()
	if dv.onDone != nil {
		dv.onDone()
	}
}

func (dv *DetailView) doSave() {
	env, ok := dv.collectEnvelope()
	if !ok {
		return
	}

	if dv.cfg.UpdateEntry == nil {
		return
	}

	go func() {
		err := dv.cfg.UpdateEntry(dv.ctx, env)
		if err != nil {
			fmt.Printf("sekeve: update entry failed: %v\n", err)
			return
		}
		gtkutil.IdleAddOnce(func() {
			dv.Hide()
			if dv.onDone != nil {
				dv.onDone()
			}
		})
	}()
}

// collectEnvelope reads form fields and creates an Envelope for update.
// Keeps the same ID, Name (potentially updated), and Type from the original.
func (dv *DetailView) collectEnvelope() (*entity.Envelope, bool) {
	if dv.env == nil {
		return nil, false
	}

	switch dv.kind {
	case detailFormLogin:
		return dv.collectLoginDetail()
	case detailFormNote:
		return dv.collectNoteDetail()
	case detailFormSecret:
		return dv.collectSecretDetail()
	}
	return nil, false
}

func (dv *DetailView) collectLoginDetail() (*entity.Envelope, bool) {
	if dv.loginSite == nil || dv.loginSite.Entry == nil ||
		dv.loginUsername == nil || dv.loginUsername.Entry == nil ||
		dv.loginPassword == nil || dv.loginPassword.Entry == nil {
		return nil, false
	}

	site := dv.loginSite.Entry.GetText()
	username := dv.loginUsername.Entry.GetText()
	password := dv.loginPassword.Entry.GetText()

	notes := ""
	if dv.loginNotes != nil && dv.loginNotes.Entry != nil {
		notes = dv.loginNotes.Entry.GetText()
	}

	login := entity.Login{Site: site, Username: username, Password: password, Notes: notes}
	payload, err := json.Marshal(login)
	if err != nil {
		return nil, false
	}

	env := &entity.Envelope{
		ID:        dv.env.ID,
		Name:      deriveLoginName(site, username),
		Type:      entity.EntryTypeLogin,
		Meta:      map[string]string{"username": username, "site": site},
		Payload:   payload,
		CreatedAt: dv.env.CreatedAt,
	}
	return env, true
}

func (dv *DetailView) collectNoteDetail() (*entity.Envelope, bool) {
	if dv.noteName == nil || dv.noteName.Entry == nil || dv.noteContent == nil {
		return nil, false
	}

	name := dv.noteName.Entry.GetText()
	if name == "" {
		return nil, false
	}

	content := getTextViewContent(dv.noteContent)

	note := entity.Note{Name: name, Content: content}
	payload, err := json.Marshal(note)
	if err != nil {
		return nil, false
	}

	env := &entity.Envelope{
		ID:        dv.env.ID,
		Name:      name,
		Type:      entity.EntryTypeNote,
		Payload:   payload,
		CreatedAt: dv.env.CreatedAt,
	}
	return env, true
}

func (dv *DetailView) collectSecretDetail() (*entity.Envelope, bool) {
	if dv.secretName == nil || dv.secretName.Entry == nil ||
		dv.secretValue == nil || dv.secretValue.Entry == nil {
		return nil, false
	}

	name := dv.secretName.Entry.GetText()
	value := dv.secretValue.Entry.GetText()

	if name == "" {
		return nil, false
	}

	secret := entity.Secret{Name: name, Value: value}
	payload, err := json.Marshal(secret)
	if err != nil {
		return nil, false
	}

	env := &entity.Envelope{
		ID:        dv.env.ID,
		Name:      name,
		Type:      entity.EntryTypeSecret,
		Payload:   payload,
		CreatedAt: dv.env.CreatedAt,
	}
	return env, true
}

// Helper functions for building focusable lists.

func entryWidget(le *widget.LabeledEntry) *gtk.Widget {
	if le != nil && le.Entry != nil {
		return &le.Entry.Widget
	}
	return nil
}

func passwordWidget(lp *widget.LabeledPassword) *gtk.Widget {
	if lp != nil && lp.Entry != nil {
		return &lp.Entry.Widget
	}
	return nil
}

func appendFieldAndCopy(out []focusring.Focusable, w *gtk.Widget, btn *gtk.Button) []focusring.Focusable {
	if w != nil {
		out = append(out, &focusableWidget{w})
	}
	out = appendCopyBtn(out, btn)
	return out
}

func appendCopyBtn(out []focusring.Focusable, btn *gtk.Button) []focusring.Focusable {
	if btn != nil {
		out = append(out, &focusableWidget{&btn.Widget})
	}
	return out
}
