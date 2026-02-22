package diffview

import (
	"fmt"
	"strings"

	sgdiff "github.com/sourcegraph/go-diff/diff"
)

func ParseUnifiedDiff(raw []byte) ([]DiffRow, error) {
	fileDiffs, err := sgdiff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, err
	}

	rows := make([]DiffRow, 0, 64)
	for _, fd := range fileDiffs {
		path := normalizePath(fd)
		rows = append(rows, DiffRow{
			Kind:    RowFileHeader,
			OldText: fmt.Sprintf("File: %s", path),
			Path:    path,
		})

		for hunkID, h := range fd.Hunks {
			rows = append(rows, DiffRow{
				Kind:    RowHunkHeader,
				OldText: formatHunkHeader(h),
				Path:    path,
				HunkID:  hunkID,
			})

			oldLn := int(h.OrigStartLine)
			newLn := int(h.NewStartLine)
			lines := splitHunkBody(h.Body)
			for i := 0; i < len(lines); {
				line := lines[i]
				if line == "" {
					i++
					continue
				}
				switch line[0] {
				case ' ':
					rows = append(rows, DiffRow{
						Kind:    RowContext,
						OldLine: linePtr(oldLn),
						NewLine: linePtr(newLn),
						OldText: line[1:],
						NewText: line[1:],
						Path:    path,
						HunkID:  hunkID,
					})
					oldLn++
					newLn++
					i++

				case '-':
					start := i
					for i < len(lines) && len(lines[i]) > 0 && lines[i][0] == '-' {
						i++
					}
					delRun := stripPrefix(lines[start:i])

					addStart := i
					for i < len(lines) && len(lines[i]) > 0 && lines[i][0] == '+' {
						i++
					}
					addRun := stripPrefix(lines[addStart:i])

					rows = append(rows, pairEditRuns(path, hunkID, &oldLn, &newLn, delRun, addRun)...)

				case '+':
					start := i
					for i < len(lines) && len(lines[i]) > 0 && lines[i][0] == '+' {
						i++
					}
					addRun := stripPrefix(lines[start:i])
					rows = append(rows, pairEditRuns(path, hunkID, &oldLn, &newLn, nil, addRun)...)

				case '\\':
					// Ignore "\ No newline at end of file" marker lines.
					i++

				default:
					return nil, fmt.Errorf("unexpected hunk line prefix %q", line)
				}
			}
		}
	}
	return rows, nil
}

func pairEditRuns(path string, hunkID int, oldLn, newLn *int, dels, adds []string) []DiffRow {
	count := maxInt(len(dels), len(adds))
	out := make([]DiffRow, 0, count)
	for i := 0; i < count; i++ {
		var oldLine *int
		var newLine *int
		oldText := ""
		newText := ""

		hasDel := i < len(dels)
		hasAdd := i < len(adds)

		if hasDel {
			oldLine = linePtr(*oldLn)
			oldText = dels[i]
			*oldLn++
		}
		if hasAdd {
			newLine = linePtr(*newLn)
			newText = adds[i]
			*newLn++
		}

		kind := RowContext
		switch {
		case hasDel && hasAdd:
			kind = RowChange
		case hasDel:
			kind = RowDelete
		case hasAdd:
			kind = RowAdd
		}

		out = append(out, DiffRow{
			Kind:    kind,
			OldLine: oldLine,
			NewLine: newLine,
			OldText: oldText,
			NewText: newText,
			Path:    path,
			HunkID:  hunkID,
		})
	}
	return out
}

func formatHunkHeader(h *sgdiff.Hunk) string {
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
	if h.Section != "" {
		header += " " + h.Section
	}
	return header
}

func normalizePath(fd *sgdiff.FileDiff) string {
	path := fd.NewName
	if path == "" || path == "/dev/null" {
		path = fd.OrigName
	}
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	return path
}

func splitHunkBody(body []byte) []string {
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func stripPrefix(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		out = append(out, line[1:])
	}
	return out
}

func linePtr(n int) *int {
	v := n
	return &v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
