package entity

import (
	"fmt"
	"net/url"
)

type Login struct {
	Site     string `json:"site"`
	Username string `json:"username"`
	Password string `json:"password"`
	Notes    string `json:"notes,omitempty"`
}

// DeriveLoginName creates a display name from site and username.
func DeriveLoginName(site, username string) string {
	domain := ExtractDomain(site)
	if domain == "" {
		domain = site
	}
	if username != "" {
		return fmt.Sprintf("%s (%s)", domain, username)
	}
	return domain
}

// ExtractDomain parses a URL or hostname and returns the host portion.
func ExtractDomain(raw string) string {
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
