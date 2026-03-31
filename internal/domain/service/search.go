package service

import (
	"strings"

	"github.com/bnema/sekeve/internal/domain/entity"
)

// SearchOpts holds criteria for filtering vault entries.
type SearchOpts struct {
	Domain string
	Email  string
	Query  string // fuzzy search across name, site, username
}

// FilterEntries returns entries matching the given search criteria.
func FilterEntries(entries []*entity.Envelope, opts SearchOpts) []*entity.Envelope {
	var matched []*entity.Envelope
	for _, e := range entries {
		if MatchesOpts(e, opts) {
			matched = append(matched, e)
		}
	}
	return matched
}

// MatchesOpts checks if an entry matches the given search options.
func MatchesOpts(e *entity.Envelope, opts SearchOpts) bool {
	if opts.Domain != "" {
		site := strings.ToLower(e.Meta["site"])
		domain := strings.ToLower(opts.Domain)
		return strings.Contains(site, domain)
	}
	if opts.Email != "" {
		return strings.EqualFold(e.Meta["username"], opts.Email)
	}
	if opts.Query != "" {
		q := strings.ToLower(opts.Query)
		name := strings.ToLower(e.Name)
		site := strings.ToLower(e.Meta["site"])
		username := strings.ToLower(e.Meta["username"])
		return strings.Contains(name, q) || strings.Contains(site, q) || strings.Contains(username, q)
	}
	return false
}
