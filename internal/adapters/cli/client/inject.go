package client

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bnema/sekeve/internal/domain/entity"
)

const sekevePrefix = "sekeve:"

type sekeveRef struct {
	Query string
	Field string // empty = primary value
}

// parseSekeveRef checks if a value is a sekeve: reference and parses it.
// Returns nil if the value is not a reference.
func parseSekeveRef(value string) *sekeveRef {
	if !strings.HasPrefix(value, sekevePrefix) {
		return nil
	}
	rest := value[len(sekevePrefix):]
	if rest == "" {
		return nil
	}

	ref := &sekeveRef{}
	if idx := strings.IndexByte(rest, '#'); idx >= 0 {
		ref.Query = rest[:idx]
		ref.Field = rest[idx+1:]
	} else {
		ref.Query = rest
	}
	return ref
}

// extractField extracts a field value from decrypted JSON payload.
// If field is empty, returns the primary value for the entry type.
func extractField(plaintext []byte, entryType string, field string) (string, error) {
	switch entryType {
	case "login":
		var login entity.Login
		if err := json.Unmarshal(plaintext, &login); err != nil {
			return "", fmt.Errorf("unmarshal login: %w", err)
		}
		if field == "" {
			return login.Password, nil
		}
		switch field {
		case "password":
			return login.Password, nil
		case "username":
			return login.Username, nil
		case "site":
			return login.Site, nil
		case "notes":
			return login.Notes, nil
		default:
			return "", fmt.Errorf("login has no field %q (valid: password, username, site, notes)", field)
		}

	case "secret":
		var secret entity.Secret
		if err := json.Unmarshal(plaintext, &secret); err != nil {
			return "", fmt.Errorf("unmarshal secret: %w", err)
		}
		if field == "" || field == "value" {
			return secret.Value, nil
		}
		if field == "name" {
			return secret.Name, nil
		}
		return "", fmt.Errorf("secret has no field %q (valid: value, name)", field)

	case "note":
		var note entity.Note
		if err := json.Unmarshal(plaintext, &note); err != nil {
			return "", fmt.Errorf("unmarshal note: %w", err)
		}
		if field == "" || field == "content" {
			return note.Content, nil
		}
		if field == "name" {
			return note.Name, nil
		}
		return "", fmt.Errorf("note has no field %q (valid: content, name)", field)

	default:
		return "", fmt.Errorf("unknown entry type %q", entryType)
	}
}
