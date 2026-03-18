package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSekeveRef(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantRef *sekeveRef
	}{
		{
			name:    "not a reference",
			value:   "plain-value",
			wantRef: nil,
		},
		{
			name:    "simple reference",
			value:   "sekeve:prod-db",
			wantRef: &sekeveRef{Query: "prod-db"},
		},
		{
			name:    "reference with field",
			value:   "sekeve:github#username",
			wantRef: &sekeveRef{Query: "github", Field: "username"},
		},
		{
			name:    "empty after prefix",
			value:   "sekeve:",
			wantRef: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseSekeveRef(tc.value)
			assert.Equal(t, tc.wantRef, got)
		})
	}
}

func TestExtractField(t *testing.T) {
	tests := []struct {
		name      string
		plainJSON string
		entryType string // "login", "secret", "note"
		field     string // empty = primary value
		want      string
		wantErr   bool
	}{
		{
			name:      "login primary (password)",
			plainJSON: `{"site":"github.com","username":"brice","password":"s3cret"}`,
			entryType: "login",
			want:      "s3cret",
		},
		{
			name:      "login username field",
			plainJSON: `{"site":"github.com","username":"brice","password":"s3cret"}`,
			entryType: "login",
			field:     "username",
			want:      "brice",
		},
		{
			name:      "login site field",
			plainJSON: `{"site":"github.com","username":"brice","password":"s3cret"}`,
			entryType: "login",
			field:     "site",
			want:      "github.com",
		},
		{
			name:      "secret primary (value)",
			plainJSON: `{"name":"db-url","value":"postgres://localhost/db"}`,
			entryType: "secret",
			want:      "postgres://localhost/db",
		},
		{
			name:      "note primary (content)",
			plainJSON: `{"name":"my-note","content":"hello world"}`,
			entryType: "note",
			want:      "hello world",
		},
		{
			name:      "unknown field",
			plainJSON: `{"site":"github.com","username":"brice","password":"s3cret"}`,
			entryType: "login",
			field:     "nonexistent",
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractField([]byte(tc.plainJSON), tc.entryType, tc.field)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
