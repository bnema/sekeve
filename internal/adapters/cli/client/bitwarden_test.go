package client

import (
	"encoding/json"
	"testing"

	"github.com/bnema/sekeve/internal/domain/entity"
)

func TestMapLoginToEnvelope_WithURIs(t *testing.T) {
	item := BitwardenItem{
		Name:  "GitHub",
		Notes: "work account",
		Login: &BitwardenLogin{
			URIs:     []BitwardenURI{{URI: "https://github.com"}, {URI: "https://api.github.com"}},
			Username: "octocat",
			Password: "secret",
		},
	}

	env, err := MapLoginToEnvelope(item)
	if err != nil {
		t.Fatalf("MapLoginToEnvelope() error = %v", err)
	}

	if env.Name != "GitHub (octocat)" {
		t.Fatalf("env.Name = %q, want %q", env.Name, "GitHub (octocat)")
	}
	if env.Type != entity.EntryTypeLogin {
		t.Fatalf("env.Type = %v, want %v", env.Type, entity.EntryTypeLogin)
	}
	wantMeta := map[string]string{"username": "octocat", "site": "https://github.com"}
	if env.Meta["username"] != wantMeta["username"] || env.Meta["site"] != wantMeta["site"] {
		t.Fatalf("env.Meta = %#v, want %#v", env.Meta, wantMeta)
	}

	var login entity.Login
	if err := json.Unmarshal(env.Payload, &login); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if login.Site != "https://github.com" || login.Username != "octocat" || login.Password != "secret" || login.Notes != "work account" {
		t.Fatalf("login = %#v, want populated login", login)
	}
}

func TestMapLoginToEnvelope_WithNilLogin(t *testing.T) {
	item := BitwardenItem{Name: "Empty Login", Notes: "no login object"}

	env, err := MapLoginToEnvelope(item)
	if err != nil {
		t.Fatalf("MapLoginToEnvelope() error = %v", err)
	}
	if env.Name != "Empty Login" {
		t.Fatalf("env.Name = %q, want %q", env.Name, "Empty Login")
	}

	if env.Meta["username"] != "" || env.Meta["site"] != "" {
		t.Fatalf("env.Meta = %#v, want empty username and site", env.Meta)
	}

	var login entity.Login
	if err := json.Unmarshal(env.Payload, &login); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if login.Site != "" || login.Username != "" || login.Password != "" || login.Notes != "no login object" {
		t.Fatalf("login = %#v, want empty login fields with notes", login)
	}
}

func TestMapLoginToEnvelope_WithNoURIs(t *testing.T) {
	item := BitwardenItem{
		Name:  "No URI",
		Notes: "missing uris",
		Login: &BitwardenLogin{
			Username: "user",
			Password: "pass",
		},
	}

	env, err := MapLoginToEnvelope(item)
	if err != nil {
		t.Fatalf("MapLoginToEnvelope() error = %v", err)
	}
	if env.Name != "No URI (user)" {
		t.Fatalf("env.Name = %q, want %q", env.Name, "No URI (user)")
	}

	if env.Meta["site"] != "" {
		t.Fatalf("env.Meta[site] = %q, want empty", env.Meta["site"])
	}

	var login entity.Login
	if err := json.Unmarshal(env.Payload, &login); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if login.Site != "" || login.Username != "user" || login.Password != "pass" || login.Notes != "missing uris" {
		t.Fatalf("login = %#v, want empty site with credentials", login)
	}
}

func TestMapLoginToEnvelope_URINormalization(t *testing.T) {
	item := BitwardenItem{
		Name: "GitHub",
		Login: &BitwardenLogin{
			URIs:     []BitwardenURI{{URI: "https://github.com/auth/login?redirect=dashboard"}},
			Username: "alice",
			Password: "pass",
		},
	}

	env, err := MapLoginToEnvelope(item)
	if err != nil {
		t.Fatalf("MapLoginToEnvelope() error = %v", err)
	}

	if env.Meta["site"] != "https://github.com" {
		t.Errorf("Meta[site] = %q, want %q", env.Meta["site"], "https://github.com")
	}

	var login entity.Login
	if err := json.Unmarshal(env.Payload, &login); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if login.Site != "https://github.com" {
		t.Errorf("login.Site = %q, want %q", login.Site, "https://github.com")
	}
}

func TestMapLoginToEnvelope_SubdomainPreserved(t *testing.T) {
	item := BitwardenItem{
		Name: "Blog",
		Login: &BitwardenLogin{
			URIs:     []BitwardenURI{{URI: "https://blog.example.com/posts/123"}},
			Username: "bob",
			Password: "pass",
		},
	}

	env, err := MapLoginToEnvelope(item)
	if err != nil {
		t.Fatalf("MapLoginToEnvelope() error = %v", err)
	}

	if env.Meta["site"] != "https://blog.example.com" {
		t.Errorf("Meta[site] = %q, want %q", env.Meta["site"], "https://blog.example.com")
	}
}

func TestNormalizeURI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/auth/login", "https://github.com"},
		{"https://blog.example.com/posts/123?a=b#frag", "https://blog.example.com"},
		{"https://example.com", "https://example.com"},
		{"", ""},
		{"not-a-url", "not-a-url"},
		{"http://localhost:8080/path", "http://localhost:8080"},
	}
	for _, tt := range tests {
		got := normalizeURI(tt.input)
		if got != tt.want {
			t.Errorf("normalizeURI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapNoteToEnvelope(t *testing.T) {
	item := BitwardenItem{Name: "Reference", Notes: "line one\nline two"}

	env, err := MapNoteToEnvelope(item)
	if err != nil {
		t.Fatalf("MapNoteToEnvelope() error = %v", err)
	}

	if env.Name != item.Name {
		t.Fatalf("env.Name = %q, want %q", env.Name, item.Name)
	}
	if env.Type != entity.EntryTypeNote {
		t.Fatalf("env.Type = %v, want %v", env.Type, entity.EntryTypeNote)
	}

	var note entity.Note
	if err := json.Unmarshal(env.Payload, &note); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if note.Name != item.Name || note.Content != item.Notes {
		t.Fatalf("note = %#v, want name/content from item", note)
	}
}

func TestMapNoteToEnvelope_WithNilNotes(t *testing.T) {
	item := BitwardenItem{Name: "Empty Note"}

	env, err := MapNoteToEnvelope(item)
	if err != nil {
		t.Fatalf("MapNoteToEnvelope() error = %v", err)
	}

	var note entity.Note
	if err := json.Unmarshal(env.Payload, &note); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if note.Name != item.Name || note.Content != "" {
		t.Fatalf("note = %#v, want empty content", note)
	}
}
