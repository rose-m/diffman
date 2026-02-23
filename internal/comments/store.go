package comments

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Store struct {
	path string
}

func NewStore(gitDir string) Store {
	return Store{path: filepath.Join(gitDir, ".diffman", "comments.json")}
}

func (s Store) Load() ([]Comment, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Comment{}, nil
		}
		return nil, err
	}

	var out []Comment
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s Store) Save(comments []Comment) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(comments, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}
