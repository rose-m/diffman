package comments

import (
	"fmt"
	"strings"
)

func ExportPlain(comments []Comment, title string) string {
	if title == "" {
		title = "Review comments"
	}

	lines := []string{title, ""}
	for i, c := range comments {
		lines = append(lines, fmt.Sprintf("%d) %s %s:%d", i+1, c.Path, c.Side.String(), c.Line))
		lines = append(lines, fmt.Sprintf("   Comment: %s", c.Body))
		if len(c.ContextBefore)+len(c.ContextAfter) > 0 {
			lines = append(lines, "   Context:")
			for _, ln := range c.ContextBefore {
				lines = append(lines, "     "+ln)
			}
			if len(c.ContextAfter) > 0 {
				lines = append(lines, "     > "+c.ContextAfter[0])
				for _, ln := range c.ContextAfter[1:] {
					lines = append(lines, "     "+ln)
				}
			}
		}
		lines = append(lines, "")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}
