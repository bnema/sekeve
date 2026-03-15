// Package cli provides re-exports of cliconfig helpers for backward compatibility.
package cli

import "github.com/bnema/sekeve/internal/adapters/cli/cliconfig"

type Config = cliconfig.Config
type SessionCache = cliconfig.SessionCache

func LoadConfig() (*cliconfig.Config, error)        { return cliconfig.LoadConfig() }
func LoadSession() (*cliconfig.SessionCache, error) { return cliconfig.LoadSession() }
func SaveSession(s *cliconfig.SessionCache) error   { return cliconfig.SaveSession(s) }
