// internal/adapters/gui/omnibox/add.go
//go:build linux

package omnibox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/bnema/puregotk/v4/gtk"
	"github.com/bnema/sekeve/internal/adapters/gui/widget"
	"github.com/bnema/sekeve/internal/domain/entity"
	"github.com/bnema/sekeve/internal/port"
	"github.com/bnema/sekeve/pkg/focusring"
	"github.com/bnema/sekeve/pkg/gtkutil"
)

// addFormKind identifies which form is currently displayed.
type addFormKind int

const (
	addFormLogin addFormKind = iota
	addFormNote
	addFormSecret
)

// AddView manages the per-category add forms.
type AddView struct {
	Root *gtk.Box

	cfg    port.OmniboxConfig
	ctx    context.Context
	kind   addFormKind
	onDone func() // called after save or cancel to switch back to search

	// Login form fields.
	loginSite     *widget.LabeledEntry
	loginUsername *widget.LabeledEntry
	loginPassword *widget.LabeledPassword

	// Note form fields.
	noteName    *widget.LabeledEntry
	noteContent *gtk.TextView

	// Secret form fields.
	secretName  *widget.LabeledEntry
	secretValue *widget.LabeledPassword

	cancelBtn *gtk.Button
	saveBtn   *gtk.Button

	callbacks []interface{}
}

// NewAddView creates the add view container. The form is built lazily
// when SetCategory is called.
func NewAddView(ctx context.Context, cfg port.OmniboxConfig, onDone func()) *AddView {
	root, _ := gtkutil.SafeNewWidget("add-root", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationVerticalValue, 6)
	})

	av := &AddView{
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
	}

	return av
}

// SetCategory rebuilds the form for the given entry type.
// EntryTypeUnspecified (All) defaults to the login form.
func (av *AddView) SetCategory(cat entity.EntryType) {
	switch cat {
	case entity.EntryTypeNote:
		av.kind = addFormNote
	case entity.EntryTypeSecret:
		av.kind = addFormSecret
	default:
		av.kind = addFormLogin
	}
	av.buildForm()
}

// Show makes the add view visible.
func (av *AddView) Show() {
	if av.Root != nil {
		av.Root.SetVisible(true)
	}
}

// Hide hides the add view.
func (av *AddView) Hide() {
	if av.Root != nil {
		av.Root.SetVisible(false)
	}
}

// Focusables returns the form fields and buttons for focus ring integration.
func (av *AddView) Focusables() []focusring.Focusable {
	var out []focusring.Focusable
	switch av.kind {
	case addFormLogin:
		if av.loginSite != nil && av.loginSite.Entry != nil {
			out = append(out, &focusableWidget{&av.loginSite.Entry.Widget})
		}
		if av.loginUsername != nil && av.loginUsername.Entry != nil {
			out = append(out, &focusableWidget{&av.loginUsername.Entry.Widget})
		}
		if av.loginPassword != nil && av.loginPassword.Entry != nil {
			out = append(out, &focusableWidget{&av.loginPassword.Entry.Widget})
		}
	case addFormNote:
		if av.noteName != nil && av.noteName.Entry != nil {
			out = append(out, &focusableWidget{&av.noteName.Entry.Widget})
		}
		if av.noteContent != nil {
			out = append(out, &focusableWidget{&av.noteContent.Widget})
		}
	case addFormSecret:
		if av.secretName != nil && av.secretName.Entry != nil {
			out = append(out, &focusableWidget{&av.secretName.Entry.Widget})
		}
		if av.secretValue != nil && av.secretValue.Entry != nil {
			out = append(out, &focusableWidget{&av.secretValue.Entry.Widget})
		}
	}
	if av.cancelBtn != nil {
		out = append(out, &focusableWidget{&av.cancelBtn.Widget})
	}
	if av.saveBtn != nil {
		out = append(out, &focusableWidget{&av.saveBtn.Widget})
	}
	return out
}

// buildForm clears the root box and populates it with the form for av.kind.
func (av *AddView) buildForm() {
	if av.Root == nil {
		return
	}

	// Clear previous children.
	av.clearRoot()

	// Reset field pointers.
	av.loginSite = nil
	av.loginUsername = nil
	av.loginPassword = nil
	av.noteName = nil
	av.noteContent = nil
	av.secretName = nil
	av.secretValue = nil
	av.cancelBtn = nil
	av.saveBtn = nil

	switch av.kind {
	case addFormLogin:
		av.buildLoginForm()
	case addFormNote:
		av.buildNoteForm()
	case addFormSecret:
		av.buildSecretForm()
	}

	av.buildButtons()
}

func (av *AddView) buildLoginForm() {
	av.loginSite = widget.NewLabeledEntry("Site", "https://example.com")
	if av.loginSite.Box != nil {
		av.Root.Append(&av.loginSite.Box.Widget)
	}

	av.loginUsername = widget.NewLabeledEntry("Username", "user@example.com")
	if av.loginUsername.Box != nil {
		av.Root.Append(&av.loginUsername.Box.Widget)
	}

	av.loginPassword = widget.NewLabeledPassword("Password", "")
	if av.loginPassword.Box != nil {
		av.Root.Append(&av.loginPassword.Box.Widget)
	}
}

func (av *AddView) buildNoteForm() {
	av.noteName = widget.NewLabeledEntry("Name", "My note")
	if av.noteName.Box != nil {
		av.Root.Append(&av.noteName.Box.Widget)
	}

	// Content label.
	contentLabel := "Content"
	label, _ := gtkutil.SafeNewWidget("note-content-label", func() *gtk.Label {
		return gtk.NewLabel(&contentLabel)
	})
	if label != nil {
		label.AddCssClass("sekeve-label")
		label.SetHalign(gtk.AlignStartValue)
		av.Root.Append(&label.Widget)
	}

	// Multiline text view in a scrolled window.
	av.noteContent, _ = gtkutil.SafeNewWidget("note-textview", func() *gtk.TextView {
		return gtk.NewTextView()
	})

	scroll, _ := gtkutil.SafeNewWidget("note-scroll", func() *gtk.ScrolledWindow {
		return gtk.NewScrolledWindow()
	})
	if scroll != nil {
		scroll.SetPolicy(gtk.PolicyNeverValue, gtk.PolicyAutomaticValue)
		scroll.SetMinContentHeight(80)
		scroll.SetMaxContentHeight(200)
		if av.noteContent != nil {
			av.noteContent.SetWrapMode(gtk.WrapWordValue)
			scroll.SetChild(&av.noteContent.Widget)
		}
		av.Root.Append(&scroll.Widget)
	}
}

func (av *AddView) buildSecretForm() {
	av.secretName = widget.NewLabeledEntry("Name", "API key name")
	if av.secretName.Box != nil {
		av.Root.Append(&av.secretName.Box.Widget)
	}

	av.secretValue = widget.NewLabeledPassword("Value", "")
	if av.secretValue.Box != nil {
		av.Root.Append(&av.secretValue.Box.Widget)
	}
}

func (av *AddView) buildButtons() {
	btnBox, _ := gtkutil.SafeNewWidget("add-btn-box", func() *gtk.Box {
		return gtk.NewBox(gtk.OrientationHorizontalValue, 8)
	})
	if btnBox == nil {
		return
	}
	btnBox.SetHalign(gtk.AlignEndValue)
	btnBox.SetMarginTop(8)

	av.cancelBtn, _ = gtkutil.SafeNewWidget("add-cancel", func() *gtk.Button {
		return gtk.NewButtonWithLabel("Cancel")
	})
	if av.cancelBtn != nil {
		av.cancelBtn.AddCssClass("sekeve-btn-cancel")
		cancelCb := func(_ gtk.Button) {
			av.doCancel()
		}
		gtkutil.RetainCallback(&av.callbacks, cancelCb)
		av.cancelBtn.ConnectClicked(&cancelCb)
		btnBox.Append(&av.cancelBtn.Widget)
	}

	av.saveBtn, _ = gtkutil.SafeNewWidget("add-save", func() *gtk.Button {
		return gtk.NewButtonWithLabel("Save")
	})
	if av.saveBtn != nil {
		av.saveBtn.AddCssClass("sekeve-btn-save")
		saveCb := func(_ gtk.Button) {
			av.doSave()
		}
		gtkutil.RetainCallback(&av.callbacks, saveCb)
		av.saveBtn.ConnectClicked(&saveCb)
		btnBox.Append(&av.saveBtn.Widget)
	}

	av.Root.Append(&btnBox.Widget)
}

// clearRoot removes all children from the root box.
func (av *AddView) clearRoot() {
	if av.Root == nil {
		return
	}
	for {
		child := av.Root.GetFirstChild()
		if child == nil {
			break
		}
		av.Root.Remove(child)
	}
}

func (av *AddView) doCancel() {
	if av.onDone != nil {
		av.onDone()
	}
}

func (av *AddView) doSave() {
	env, ok := av.collectEnvelope()
	if !ok {
		return
	}

	if av.cfg.AddEntry == nil {
		return
	}

	go func() {
		err := av.cfg.AddEntry(av.ctx, env)
		if err != nil {
			fmt.Printf("sekeve: add entry failed: %v\n", err)
			return
		}
		gtkutil.IdleAddOnce(func() {
			if av.onDone != nil {
				av.onDone()
			}
		})
	}()
}

// collectEnvelope reads form fields and creates an Envelope.
// Returns false if required fields are empty.
func (av *AddView) collectEnvelope() (*entity.Envelope, bool) {
	switch av.kind {
	case addFormLogin:
		return av.collectLogin()
	case addFormNote:
		return av.collectNote()
	case addFormSecret:
		return av.collectSecret()
	}
	return nil, false
}

func (av *AddView) collectLogin() (*entity.Envelope, bool) {
	if av.loginSite == nil || av.loginSite.Entry == nil ||
		av.loginUsername == nil || av.loginUsername.Entry == nil ||
		av.loginPassword == nil || av.loginPassword.Entry == nil {
		return nil, false
	}

	site := av.loginSite.Entry.GetText()
	username := av.loginUsername.Entry.GetText()
	password := av.loginPassword.Entry.GetText()

	if site == "" && username == "" {
		return nil, false
	}

	login := entity.Login{Site: site, Username: username, Password: password, Notes: ""}
	payload, err := json.Marshal(login)
	if err != nil {
		return nil, false
	}

	env := &entity.Envelope{
		Name:    deriveLoginName(site, username),
		Type:    entity.EntryTypeLogin,
		Meta:    map[string]string{"username": username, "site": site},
		Payload: payload,
	}
	return env, true
}

func (av *AddView) collectNote() (*entity.Envelope, bool) {
	if av.noteName == nil || av.noteName.Entry == nil || av.noteContent == nil {
		return nil, false
	}

	name := av.noteName.Entry.GetText()
	if name == "" {
		return nil, false
	}

	content := getTextViewContent(av.noteContent)

	note := entity.Note{Name: name, Content: content}
	payload, err := json.Marshal(note)
	if err != nil {
		return nil, false
	}

	env := &entity.Envelope{
		Name:    name,
		Type:    entity.EntryTypeNote,
		Payload: payload,
	}
	return env, true
}

func (av *AddView) collectSecret() (*entity.Envelope, bool) {
	if av.secretName == nil || av.secretName.Entry == nil ||
		av.secretValue == nil || av.secretValue.Entry == nil {
		return nil, false
	}

	name := av.secretName.Entry.GetText()
	value := av.secretValue.Entry.GetText()

	if name == "" {
		return nil, false
	}

	secret := entity.Secret{Name: name, Value: value}
	payload, err := json.Marshal(secret)
	if err != nil {
		return nil, false
	}

	env := &entity.Envelope{
		Name:    name,
		Type:    entity.EntryTypeSecret,
		Payload: payload,
	}
	return env, true
}

// getTextViewContent extracts text from a TextView's buffer.
func getTextViewContent(tv *gtk.TextView) string {
	buf := tv.GetBuffer()
	if buf == nil {
		return ""
	}
	var start, end gtk.TextIter
	buf.GetStartIter(&start)
	buf.GetEndIter(&end)
	return buf.GetText(&start, &end, false)
}

// deriveLoginName creates a display name from site and username.
func deriveLoginName(site, username string) string {
	domain := extractDomain(site)
	if domain == "" {
		domain = site
	}
	if username != "" {
		return fmt.Sprintf("%s (%s)", domain, username)
	}
	return domain
}

// extractDomain parses a URL or hostname and returns the host portion.
func extractDomain(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := u.Host
	if host == "" {
		u2, _ := url.Parse("https://" + raw)
		if u2 != nil {
			host = u2.Host
		}
	}
	if host == "" {
		return raw
	}
	return host
}
