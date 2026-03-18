package client

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type envLine struct {
	Key   string
	Value string
}

// parseDotenv reads a .env file and returns key-value pairs.
// Skips comments (#) and blank lines. Strips optional "export " prefix.
// Supports single/double quoted values (quotes are removed).
// Inline comments are NOT stripped — the value is everything after '='.
func parseDotenv(r io.Reader) ([]envLine, error) {
	var lines []envLine
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")

		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			return nil, fmt.Errorf("line %d: invalid format (no '='): %s", lineNum, line)
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = unquote(val)

		lines = append(lines, envLine{Key: key, Value: val})
	}
	return lines, scanner.Err()
}

// unquote removes matching single or double quotes from a string.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
