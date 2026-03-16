package styles

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/bnema/sekeve/internal/domain/entity"
)

var (
	ColorPrimary   = lipgloss.Color("99")
	ColorSuccess   = lipgloss.Color("82")
	ColorError     = lipgloss.Color("196")
	ColorWarning   = lipgloss.Color("214")
	ColorMuted     = lipgloss.Color("245")
	ColorHighlight = lipgloss.Color("212")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorError)

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	LabelStyle = lipgloss.NewStyle().
			Bold(true).
			Width(12)

	ValueStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight)
)

func RenderError(w io.Writer, err error) error {
	if _, werr := lipgloss.Fprint(w, ErrorStyle.Render("Error: "+err.Error())); werr != nil {
		return werr
	}
	if _, werr := fmt.Fprintln(w); werr != nil {
		return werr
	}
	return nil
}

func RenderSuccess(w io.Writer, msg string) error {
	if _, werr := lipgloss.Fprint(w, SuccessStyle.Render("✓ "+msg)); werr != nil {
		return werr
	}
	if _, werr := fmt.Fprintln(w); werr != nil {
		return werr
	}
	return nil
}

// Field is a labeled key-value pair for ordered display in RenderEntry.
type Field struct {
	Label string
	Value string
}

func RenderEntry(w io.Writer, env *entity.Envelope, fields []Field) error {
	if _, werr := lipgloss.Fprintln(w, TitleStyle.Render(env.Name)+" "+MutedStyle.Render("("+env.Type.String()+")")); werr != nil {
		return werr
	}
	for _, f := range fields {
		if _, werr := lipgloss.Fprintln(w, LabelStyle.Render(f.Label)+ValueStyle.Render(f.Value)); werr != nil {
			return werr
		}
	}
	return nil
}

func RenderTable(w io.Writer, entries []*entity.Envelope) error {
	headerStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Align(lipgloss.Center)

	cellStyle := lipgloss.NewStyle().Padding(0, 1)

	var rows [][]string
	for _, e := range entries {
		rows = append(rows, []string{
			e.Name,
			e.Type.String(),
			e.CreatedAt.Format(time.RFC3339),
		})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ColorPrimary)).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Headers("NAME", "TYPE", "CREATED").
		Rows(rows...)

	if _, werr := lipgloss.Fprintln(w, t); werr != nil {
		return werr
	}
	return nil
}

func RenderJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
