package comments

import (
	"fmt"
	"strings"
)

func ExportPlain(comments []Comment, title string) string {
	if title == "" {
		title = "Review comments:"
	}
	if !strings.HasSuffix(title, ":") {
		title += ":"
	}

	lines := []string{title, ""}
	for i, c := range comments {
		body := strings.ReplaceAll(strings.TrimSpace(c.Body), "\n", " / ")
		lines = append(lines, fmt.Sprintf("%d) %s %s:%d: %s", i+1, c.Path, c.Side.String(), c.Line, body))
		if ctx := exportContextLines(c); len(ctx) > 0 {
			lines = append(lines, "```")
			lines = append(lines, ctx...)
			lines = append(lines, "```")
		}
		lines = append(lines, "")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func exportContextLines(c Comment) []string {
	out := make([]string, 0, len(c.ContextBefore)+len(c.ContextAfter))
	out = append(out, c.ContextBefore...)
	if len(c.ContextAfter) > 0 {
		out = append(out, "> "+c.ContextAfter[0])
		out = append(out, c.ContextAfter[1:]...)
	}
	return out
}
