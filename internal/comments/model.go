package comments

import "time"

type Side int

const (
	SideOld Side = iota
	SideNew
)

type Comment struct {
	Path          string    `json:"path"`
	Side          Side      `json:"side"`
	Line          int       `json:"line"`
	Body          string    `json:"body"`
	CreatedAt     time.Time `json:"created_at"`
	HunkHeader    string    `json:"hunk_header"`
	ContextBefore []string  `json:"context_before"`
	ContextAfter  []string  `json:"context_after"`
}

func AnchorKey(path string, side Side, line int) string {
	return path + ":" + side.String() + ":" + fmtInt(line)
}

func (s Side) String() string {
	if s == SideOld {
		return "old"
	}
	return "new"
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 16)
	for n > 0 {
		d := byte(n%10) + '0'
		buf = append([]byte{d}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
