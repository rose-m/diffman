package diffview

import "fmt"

func RenderSideBySide(rows []DiffRow, width int) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, fmt.Sprintf("%v | %s || %s", row.Kind, row.OldText, row.NewText))
	}
	_ = width
	return out
}
