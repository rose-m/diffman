package git

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"diffman/internal/util"
)

// FileItem is one changed file from git status.
type FileItem struct {
	Path        string
	Status      string
	HasStaged   bool
	HasUnstaged bool
}

type StatusService interface {
	ListChangedFiles(ctx context.Context, cwd string) ([]FileItem, error)
}

type statusService struct{}

func NewStatusService() StatusService {
	return statusService{}
}

func (statusService) ListChangedFiles(ctx context.Context, cwd string) ([]FileItem, error) {
	out, err := util.Run(ctx, cwd, "git", "status", "--porcelain=v2", "--untracked-files=all", "-z")
	if err != nil {
		return nil, err
	}

	items, err := parsePorcelainV2Z([]byte(out))
	if err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})

	return items, nil
}

func parsePorcelainV2Z(data []byte) ([]FileItem, error) {
	records := bytes.Split(data, []byte{0})
	items := make([]FileItem, 0, len(records))

	for i := 0; i < len(records); i++ {
		rec := string(records[i])
		if rec == "" {
			continue
		}

		switch rec[0] {
		case '1', 'u':
			fields := strings.Fields(rec)
			if len(fields) < 2 {
				return nil, fmt.Errorf("unexpected porcelain record: %q", rec)
			}
			path := fields[len(fields)-1]
			item := itemFromXY(path, fields[1])
			items = append(items, item)

		case '2':
			fields := strings.Fields(rec)
			if len(fields) < 2 {
				return nil, fmt.Errorf("unexpected rename/copy record: %q", rec)
			}
			path := fields[len(fields)-1]
			item := itemFromXY(path, fields[1])
			items = append(items, item)
			if i+1 < len(records) {
				i++ // consume the original path record emitted for -z rename/copy entries
			}

		case '?':
			path := strings.TrimPrefix(rec, "? ")
			items = append(items, FileItem{
				Path:        path,
				Status:      "??",
				HasStaged:   false,
				HasUnstaged: true,
			})

		case '!':
			continue

		case '#':
			continue

		default:
			return nil, fmt.Errorf("unknown porcelain record: %q", rec)
		}
	}

	return items, nil
}

func itemFromXY(path, xy string) FileItem {
	hasStaged := len(xy) > 0 && xy[0] != '.'
	hasUnstaged := len(xy) > 1 && xy[1] != '.'
	status := strings.TrimSpace(xy)
	if status == "" {
		status = ".."
	}

	return FileItem{
		Path:        path,
		Status:      status,
		HasStaged:   hasStaged,
		HasUnstaged: hasUnstaged,
	}
}
