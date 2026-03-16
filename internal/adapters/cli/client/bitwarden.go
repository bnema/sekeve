package client

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/bnema/sekeve/internal/domain/entity"
)

const (
	bwTypeLogin      = 1
	bwTypeSecureNote = 2
	bwTypeCard       = 3
	bwTypeIdentity   = 4
	bwTypeSSHKey     = 5
)

type BitwardenExport struct {
	Encrypted bool            `json:"encrypted"`
	Folders   json.RawMessage `json:"folders"`
	Items     []BitwardenItem `json:"items"`
}

type BitwardenItem struct {
	Type  int             `json:"type"`
	Name  string          `json:"name"`
	Notes string          `json:"notes"`
	Login *BitwardenLogin `json:"login"`
}

type BitwardenLogin struct {
	URIs     []BitwardenURI `json:"uris"`
	Username string         `json:"username"`
	Password string         `json:"password"`
}

type BitwardenURI struct {
	URI string `json:"uri"`
}

// normalizeURI strips path, query, and fragment from a URI, keeping scheme and host.
// Subdomains are preserved. Returns the original string if parsing fails.
func normalizeURI(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func MapLoginToEnvelope(item BitwardenItem) (*entity.Envelope, error) {
	login := entity.Login{Notes: item.Notes}

	if item.Login != nil {
		login.Username = item.Login.Username
		login.Password = item.Login.Password
		if len(item.Login.URIs) > 0 {
			login.Site = normalizeURI(item.Login.URIs[0].URI)
		}
	}

	name := item.Name
	if login.Username != "" {
		name = fmt.Sprintf("%s (%s)", item.Name, login.Username)
	}

	payload, err := json.Marshal(login)
	if err != nil {
		return nil, err
	}

	return &entity.Envelope{
		Name: name,
		Type: entity.EntryTypeLogin,
		Meta: map[string]string{
			"username": login.Username,
			"site":     login.Site,
		},
		Payload: payload,
	}, nil
}

func MapNoteToEnvelope(item BitwardenItem) (*entity.Envelope, error) {
	note := entity.Note{
		Name:    item.Name,
		Content: item.Notes,
	}

	payload, err := json.Marshal(note)
	if err != nil {
		return nil, err
	}

	return &entity.Envelope{
		Name:    item.Name,
		Type:    entity.EntryTypeNote,
		Payload: payload,
	}, nil
}
