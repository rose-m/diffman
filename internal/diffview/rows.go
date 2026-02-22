package diffview

type Side int

const (
	SideOld Side = iota
	SideNew
)

type RowKind int

const (
	RowContext RowKind = iota
	RowDelete
	RowAdd
	RowChange
	RowHunkHeader
	RowFileHeader
)

type DiffRow struct {
	Kind    RowKind
	OldLine *int
	NewLine *int
	OldText string
	NewText string
	Path    string
	HunkID  int
}
