package submissions

import (
	"strings"
	"testing"
)

func TestPrepareAcceptsValidInput(t *testing.T) {
	p, fields := Prepare([]FileInput{{Path: "src/expenses.js", Content: "const expenses = [];"}}, "  first try ")
	if len(fields) != 0 {
		t.Fatalf("unexpected errors: %v", fields)
	}
	if p.TotalBytes != len("const expenses = [];") || p.Note != "first try" {
		t.Fatalf("unexpected result: %+v", p)
	}
	if p.Files[0].Language != "javascript" {
		t.Fatalf("language not derived: %q", p.Files[0].Language)
	}
	if len(p.SHA256) != 64 {
		t.Fatalf("hash looks wrong: %q", p.SHA256)
	}
}

func TestPrepareRejectsBadPaths(t *testing.T) {
	bad := []string{"", "../secret.js", "/etc/passwd.js", "a//b.js", "a\\b.js", "run.exe", "noext", "dir/./x.js"}
	for _, p := range bad {
		_, fields := Prepare([]FileInput{{Path: p, Content: "x"}}, "")
		if len(fields) == 0 {
			t.Errorf("path %q should have been rejected", p)
		}
	}
}

func TestPrepareRejectsBinaryAndEmpty(t *testing.T) {
	if _, f := Prepare([]FileInput{{Path: "a.js", Content: "a\x00b"}}, ""); len(f) == 0 {
		t.Error("NUL byte should be rejected")
	}
	if _, f := Prepare([]FileInput{{Path: "a.js", Content: "\xff\xfe"}}, ""); len(f) == 0 {
		t.Error("invalid UTF-8 should be rejected")
	}
	if _, f := Prepare([]FileInput{{Path: "a.js", Content: ""}}, ""); len(f) == 0 {
		t.Error("all-empty submission should be rejected")
	}
}

func TestPrepareLimits(t *testing.T) {
	many := make([]FileInput, 11)
	for i := range many {
		many[i] = FileInput{Path: "f" + string(rune('a'+i)) + ".js", Content: "x"}
	}
	if _, f := Prepare(many, ""); len(f) == 0 {
		t.Error("11 files should be rejected")
	}
	if _, f := Prepare(nil, ""); len(f) == 0 {
		t.Error("zero files should be rejected")
	}
	big := strings.Repeat("a", MaxTotalBytes+1)
	if _, f := Prepare([]FileInput{{Path: "a.js", Content: big}}, ""); len(f) == 0 {
		t.Error("oversize submission should be rejected")
	}
	exact := strings.Repeat("a", MaxTotalBytes)
	if _, f := Prepare([]FileInput{{Path: "a.js", Content: exact}}, ""); len(f) != 0 {
		t.Errorf("exactly 100 KB should be accepted: %v", f)
	}
}

func TestPrepareRejectsDuplicatePaths(t *testing.T) {
	_, f := Prepare([]FileInput{{Path: "a.js", Content: "1"}, {Path: "A.JS", Content: "2"}}, "")
	if len(f) == 0 {
		t.Error("duplicate paths (any case) should be rejected")
	}
}

func TestHashIgnoresFileOrder(t *testing.T) {
	a := FileInput{Path: "a.js", Content: "one"}
	b := FileInput{Path: "b.js", Content: "two"}
	p1, _ := Prepare([]FileInput{a, b}, "")
	p2, _ := Prepare([]FileInput{b, a}, "")
	if p1.SHA256 != p2.SHA256 {
		t.Error("hash should not depend on file order")
	}
	c := FileInput{Path: "b.js", Content: "changed"}
	p3, _ := Prepare([]FileInput{a, c}, "")
	if p1.SHA256 == p3.SHA256 {
		t.Error("hash should change when content changes")
	}
}
