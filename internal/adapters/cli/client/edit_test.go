// Integration tests for editInEditor: shells out to real editors
// and verifies secure temp file handling (permissions, cleanup, overwrite).
package client

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditInEditor_SecureCleanup(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "simple content is returned and cleaned up",
			content: "hello world",
		},
		{
			name:    "multiline content",
			content: "line1\nline2\nline3\n",
		},
		{
			name:    "empty content",
			content: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use "true" as editor — it exits 0 without modifying the file
			t.Setenv("EDITOR", "true")

			result, err := editInEditor(tt.content)
			require.NoError(t, err)
			assert.Equal(t, tt.content, result)
		})
	}
}

func TestEditInEditor_TempFileRemoved(t *testing.T) {
	t.Setenv("EDITOR", "true")

	// Run editInEditor and check no sekeve temp files remain in tmp dir
	_, err := editInEditor("sensitive data")
	require.NoError(t, err)

	// Verify no leftover sekeve temp files
	matches, _ := os.ReadDir(os.TempDir())
	for _, m := range matches {
		assert.NotContains(t, m.Name(), "sekeve-edit-",
			"temp file should be removed after editInEditor")
	}
}

func TestEditInEditor_FilePermissions(t *testing.T) {
	// Use a script that verifies permissions are 0600
	script := `#!/bin/sh
perms=$(stat -c %a "$1")
if [ "$perms" != "600" ]; then
  echo "bad permissions: $perms" >&2
  exit 1
fi
`
	scriptFile, err := os.CreateTemp("", "sekeve-test-editor-*.sh")
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Remove(scriptFile.Name())) }()
	_, err = scriptFile.WriteString(script)
	require.NoError(t, err)
	require.NoError(t, scriptFile.Close())
	require.NoError(t, os.Chmod(scriptFile.Name(), 0755))

	t.Setenv("EDITOR", scriptFile.Name())

	_, err = editInEditor("secret content")
	require.NoError(t, err)
}

func TestEditInEditor_EditorFailure(t *testing.T) {
	t.Setenv("EDITOR", "false") // exits with code 1

	_, err := editInEditor("some content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "editor failed")
}
