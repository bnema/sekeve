package client

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDotenv(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []envLine
		wantErr bool
	}{
		{
			name:  "simple key=value",
			input: "FOO=bar\nBAZ=qux",
			want: []envLine{
				{Key: "FOO", Value: "bar"},
				{Key: "BAZ", Value: "qux"},
			},
		},
		{
			name:  "sekeve reference",
			input: "DB_URL=sekeve:prod-db",
			want: []envLine{
				{Key: "DB_URL", Value: "sekeve:prod-db"},
			},
		},
		{
			name:  "sekeve reference with field",
			input: "USER=sekeve:github#username",
			want: []envLine{
				{Key: "USER", Value: "sekeve:github#username"},
			},
		},
		{
			name:  "skip comments and blank lines",
			input: "# comment\n\nFOO=bar\n  # indented comment\n",
			want: []envLine{
				{Key: "FOO", Value: "bar"},
			},
		},
		{
			name:  "quoted values",
			input: `FOO="bar baz"` + "\n" + `QUX='hello world'`,
			want: []envLine{
				{Key: "FOO", Value: "bar baz"},
				{Key: "QUX", Value: "hello world"},
			},
		},
		{
			name:  "value with equals sign",
			input: "URL=postgres://host:5432/db?sslmode=require",
			want: []envLine{
				{Key: "URL", Value: "postgres://host:5432/db?sslmode=require"},
			},
		},
		{
			name:  "empty value",
			input: "EMPTY=",
			want: []envLine{
				{Key: "EMPTY", Value: ""},
			},
		},
		{
			name:  "export prefix stripped",
			input: "export FOO=bar",
			want: []envLine{
				{Key: "FOO", Value: "bar"},
			},
		},
		{
			name:    "invalid line no equals",
			input:   "BROKEN",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDotenv(strings.NewReader(tc.input))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
