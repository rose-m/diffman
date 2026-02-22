package diffview

import "testing"

func TestParseUnifiedDiffPairsDeleteAndAddRuns(t *testing.T) {
	raw := []byte(`diff --git a/sample.txt b/sample.txt
index 1111111..2222222 100644
--- a/sample.txt
+++ b/sample.txt
@@ -1,4 +1,5 @@
 keep
-oldA
-oldB
+newA
+newB
+newC
 tail
`)

	rows, err := ParseUnifiedDiff(raw)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff returned error: %v", err)
	}
	if len(rows) < 6 {
		t.Fatalf("expected at least 6 rows, got %d", len(rows))
	}

	content := rows[1:]
	if got, want := content[0].Kind, RowContext; got != want {
		t.Fatalf("row 0 kind = %v, want %v", got, want)
	}
	if got, want := content[1].Kind, RowChange; got != want {
		t.Fatalf("row 1 kind = %v, want %v", got, want)
	}
	if got, want := content[2].Kind, RowChange; got != want {
		t.Fatalf("row 2 kind = %v, want %v", got, want)
	}
	if got, want := content[3].Kind, RowAdd; got != want {
		t.Fatalf("row 3 kind = %v, want %v", got, want)
	}
	if got, want := content[4].Kind, RowContext; got != want {
		t.Fatalf("row 4 kind = %v, want %v", got, want)
	}

	assertLine(t, content[0].OldLine, 1)
	assertLine(t, content[0].NewLine, 1)
	assertLine(t, content[1].OldLine, 2)
	assertLine(t, content[1].NewLine, 2)
	assertLine(t, content[2].OldLine, 3)
	assertLine(t, content[2].NewLine, 3)
	if content[3].OldLine != nil {
		t.Fatalf("expected add row old line to be nil, got %d", *content[3].OldLine)
	}
	assertLine(t, content[3].NewLine, 4)
	assertLine(t, content[4].OldLine, 4)
	assertLine(t, content[4].NewLine, 5)
}

func TestParseUnifiedDiffHandlesNewFile(t *testing.T) {
	raw := []byte(`diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..3b18e13
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,2 @@
+line1
+line2
`)

	rows, err := ParseUnifiedDiff(raw)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff returned error: %v", err)
	}
	if len(rows) < 3 {
		t.Fatalf("expected at least 3 rows, got %d", len(rows))
	}

	if rows[0].Kind != RowHunkHeader {
		t.Fatalf("first row kind = %v, want RowHunkHeader", rows[0].Kind)
	}
	if rows[1].Kind != RowAdd || rows[2].Kind != RowAdd {
		t.Fatalf("expected add rows, got %v and %v", rows[1].Kind, rows[2].Kind)
	}
	if rows[1].OldLine != nil || rows[2].OldLine != nil {
		t.Fatalf("expected old lines to be nil for new file additions")
	}
	assertLine(t, rows[1].NewLine, 1)
	assertLine(t, rows[2].NewLine, 2)
}

func assertLine(t *testing.T, got *int, want int) {
	t.Helper()
	if got == nil {
		t.Fatalf("line = nil, want %d", want)
	}
	if *got != want {
		t.Fatalf("line = %d, want %d", *got, want)
	}
}
