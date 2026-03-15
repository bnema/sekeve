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

func RenderError(w io.Writer, err error) {
	lipgloss.Fprint(w, ErrorStyle.Render("Error: "+err.Error()))
	fmt.Fprintln(w)
}

func RenderSuccess(w io.Writer, msg string) {
	lipgloss.Fprint(w, SuccessStyle.Render("✓ "+msg))
	fmt.Fprintln(w)
}

func RenderEntry(w io.Writer, env *entity.Envelope, fields map[string]string) {
	lipgloss.Fprintln(w, TitleStyle.Render(env.Name)+" "+MutedStyle.Render("("+env.Type.String()+")"))
	for label, value := range fields {
		lipgloss.Fprintln(w, LabelStyle.Render(label)+ValueStyle.Render(value))
	}
}

func RenderTable(w io.Writer, entries []*entity.Envelope) {
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
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Headers("NAME", "TYPE", "CREATED").
		Rows(rows...)

	lipgloss.Fprintln(w, t)
}

func RenderJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
