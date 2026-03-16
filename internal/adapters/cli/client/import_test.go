package client

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bnema/sekeve/internal/domain/entity"
)

// mockVault implements VaultImporter for testing.
type mockVault struct {
	existing []*entity.Envelope
	listErr  error
	addErr   error
}

func (m *mockVault) ListEntries(_ context.Context, _ entity.EntryType) ([]*entity.Envelope, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.existing, nil
}

func (m *mockVault) AddEntry(_ context.Context, _ *entity.Envelope) error {
	return m.addErr
}

func TestProcessImport_ListEntriesFails(t *testing.T) {
	vault := &mockVault{listErr: fmt.Errorf("connection refused")}
	export := BitwardenExport{Items: []BitwardenItem{{Type: 1, Name: "test"}}}
	var buf bytes.Buffer

	_, err := processImport(context.Background(), vault, export, &buf)
	if err == nil {
		t.Fatal("expected error when ListEntries fails")
	}
	if !strings.Contains(err.Error(), "failed to list existing entries") {
		t.Errorf("error = %q, want to contain 'failed to list existing entries'", err.Error())
	}
}

func TestProcessImport_ServerDuplicate(t *testing.T) {
	vault := &mockVault{
		existing: []*entity.Envelope{{
			Name: "GitHub (a)",
			Type: entity.EntryTypeLogin,
			Meta: map[string]string{"site": "", "username": "a"},
		}},
	}
	export := BitwardenExport{
		Items: []BitwardenItem{
			{Type: 1, Name: "GitHub", Login: &BitwardenLogin{Username: "a", Password: "b"}},
		},
	}
	var buf bytes.Buffer

	result, err := processImport(context.Background(), vault, export, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", result.Duplicates)
	}
	if result.Imported != 0 {
		t.Errorf("Imported = %d, want 0", result.Imported)
	}
	if !strings.Contains(buf.String(), "skipped duplicate") {
		t.Errorf("output = %q, want to contain 'skipped duplicate'", buf.String())
	}
}

func TestProcessImport_InFileDuplicate(t *testing.T) {
	vault := &mockVault{}
	export := BitwardenExport{
		Items: []BitwardenItem{
			{Type: 1, Name: "GitHub", Login: &BitwardenLogin{Username: "a", Password: "b"}},
			{Type: 1, Name: "GitHub", Login: &BitwardenLogin{Username: "a", Password: "d"}},
		},
	}
	var buf bytes.Buffer

	result, err := processImport(context.Background(), vault, export, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 1 {
		t.Errorf("Imported = %d, want 1", result.Imported)
	}
	if result.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", result.Duplicates)
	}
}

func TestProcessImport_UnsupportedTypes(t *testing.T) {
	vault := &mockVault{}
	export := BitwardenExport{
		Items: []BitwardenItem{
			{Type: bwTypeCard, Name: "My Card"},
			{Type: bwTypeIdentity, Name: "My ID"},
			{Type: bwTypeSSHKey, Name: "My Key"},
			{Type: 99, Name: "Unknown"},
		},
	}
	var buf bytes.Buffer

	result, err := processImport(context.Background(), vault, export, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Unsupported != 4 {
		t.Errorf("Unsupported = %d, want 4", result.Unsupported)
	}
	if result.Imported != 0 {
		t.Errorf("Imported = %d, want 0", result.Imported)
	}
}

func TestProcessImport_EmptyName(t *testing.T) {
	vault := &mockVault{}
	export := BitwardenExport{
		Items: []BitwardenItem{
			{Type: 1, Name: ""},
			{Type: 2, Name: "   "},
		},
	}
	var buf bytes.Buffer

	result, err := processImport(context.Background(), vault, export, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Invalid != 2 {
		t.Errorf("Invalid = %d, want 2", result.Invalid)
	}
	if !strings.Contains(buf.String(), "skipped item with empty name") {
		t.Errorf("output = %q, want to contain 'skipped item with empty name'", buf.String())
	}
}

func TestProcessImport_MixedImport(t *testing.T) {
	vault := &mockVault{
		existing: []*entity.Envelope{{
			Name: "Login1 (a)",
			Type: entity.EntryTypeLogin,
			Meta: map[string]string{"site": "", "username": "a"},
		}},
	}
	export := BitwardenExport{
		Items: []BitwardenItem{
			{Type: 1, Name: "Login1", Login: &BitwardenLogin{Username: "a", Password: "b"}},
			{Type: 2, Name: "Note1", Notes: "content"},
			{Type: 1, Name: "Existing", Login: &BitwardenLogin{Username: "c", Password: "d"}},
			{Type: 3, Name: "Card1"},
			{Type: 1, Name: ""},
			{Type: 1, Name: "Login1", Login: &BitwardenLogin{Username: "a", Password: "f"}},
		},
	}
	var buf bytes.Buffer

	result, err := processImport(context.Background(), vault, export, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 2 {
		t.Errorf("Imported = %d, want 2", result.Imported)
	}
	if result.Duplicates != 2 {
		t.Errorf("Duplicates = %d, want 2 (1 server + 1 in-file)", result.Duplicates)
	}
	if result.Unsupported != 1 {
		t.Errorf("Unsupported = %d, want 1", result.Unsupported)
	}
	if result.Invalid != 1 {
		t.Errorf("Invalid = %d, want 1", result.Invalid)
	}
}

func TestProcessImport_AddEntryFails(t *testing.T) {
	vault := &mockVault{addErr: fmt.Errorf("encryption failed")}
	export := BitwardenExport{
		Items: []BitwardenItem{
			{Type: 1, Name: "Login1", Login: &BitwardenLogin{Username: "a", Password: "b"}},
			{Type: 2, Name: "Note1", Notes: "content"},
		},
	}
	var buf bytes.Buffer

	result, err := processImport(context.Background(), vault, export, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 2 {
		t.Errorf("Failed = %d, want 2", result.Failed)
	}
	if result.Imported != 0 {
		t.Errorf("Imported = %d, want 0", result.Imported)
	}
	if !strings.Contains(buf.String(), "failed to import") {
		t.Errorf("output = %q, want to contain 'failed to import'", buf.String())
	}
}

func TestProcessImport_EmptyItems(t *testing.T) {
	vault := &mockVault{}
	export := BitwardenExport{Items: []BitwardenItem{}}
	var buf bytes.Buffer

	result, err := processImport(context.Background(), vault, export, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 0 {
		t.Errorf("Imported = %d, want 0", result.Imported)
	}
	if !strings.Contains(buf.String(), "nothing to import") {
		t.Errorf("output = %q, want to contain 'nothing to import'", buf.String())
	}
}

func TestDeduplicationKey_LoginWithNilMetaFallsBackToName(t *testing.T) {
	env := &entity.Envelope{
		Type: entity.EntryTypeLogin,
		Name: "GitHub",
	}

	got := deduplicationKey(env)
	want := fmt.Sprintf("%d:%s", entity.EntryTypeLogin, "GitHub")
	if got != want {
		t.Fatalf("deduplicationKey() = %q, want %q", got, want)
	}
}
