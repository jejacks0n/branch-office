package git

import (
	"strings"
	"testing"
)

func TestParseDiff(t *testing.T) {
	rawDiff := `diff --git a/hello.txt b/hello.txt
index e69de29..4995f97 100644
--- a/hello.txt
+++ b/hello.txt
@@ -1,4 +1,5 @@
 line 1
-line 2
+line 2 modified
+line 2.5 added
 line 3
 line 4
@@ -10,3 +11,4 @@
 line 10
 line 11
+line 12
 line 13
`

	diffs := ParseDiff(rawDiff)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 file diff, got %d", len(diffs))
	}

	fd := diffs[0]
	if fd.OldPath != "hello.txt" || fd.NewPath != "hello.txt" {
		t.Errorf("expected paths hello.txt, got %s -> %s", fd.OldPath, fd.NewPath)
	}

	if fd.Additions != 3 {
		t.Errorf("expected 3 additions, got %d", fd.Additions)
	}

	if fd.Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", fd.Deletions)
	}

	if len(fd.Hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(fd.Hunks))
	}

	hunk0 := fd.Hunks[0]
	if hunk0.OldStart != 1 || hunk0.OldLength != 4 || hunk0.NewStart != 1 || hunk0.NewLength != 5 {
		t.Errorf("unexpected hunk 0 bounds: -%d,%d +%d,%d", hunk0.OldStart, hunk0.OldLength, hunk0.NewStart, hunk0.NewLength)
	}

	if !strings.Contains(hunk0.Patch, "diff --git a/hello.txt b/hello.txt") {
		t.Errorf("hunk patch missing file header: %s", hunk0.Patch)
	}

	if !strings.Contains(hunk0.Patch, "@@ -1,4 +1,5 @@") {
		t.Errorf("hunk patch missing hunk header: %s", hunk0.Patch)
	}

	if !strings.Contains(hunk0.Patch, "+line 2.5 added") {
		t.Errorf("hunk patch missing added line: %s", hunk0.Patch)
	}

	hunk1 := fd.Hunks[1]
	if hunk1.OldStart != 10 || hunk1.NewStart != 11 {
		t.Errorf("unexpected hunk 1 bounds: -%d +%d", hunk1.OldStart, hunk1.NewStart)
	}
}
