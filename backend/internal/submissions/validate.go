package submissions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	MaxFiles      = 10
	MaxTotalBytes = 100 * 1024
	MaxPathBytes  = 200
	MaxNoteRunes  = 2000
)

// allowedExt maps a file extension to the language label stored with the file.
var allowedExt = map[string]string{
	".js": "javascript", ".jsx": "javascript", ".mjs": "javascript",
	".ts": "typescript", ".tsx": "typescript",
	".go":   "go",
	".json": "json", ".md": "markdown", ".txt": "text", ".html": "html",
	".css": "css", ".sql": "sql", ".yaml": "yaml", ".yml": "yaml",
	".mod": "text", ".sum": "text",
}

type FileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type File struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Content  string `json:"content"`
}

// Prepared is a validated submission, ready to store.
type Prepared struct {
	Files      []File
	TotalBytes int
	SHA256     string
	Note       string
}

// Prepare validates the input and computes size and hash.
// On failure it returns a map of field name to message.
func Prepare(in []FileInput, note string) (Prepared, map[string]string) {
	fields := map[string]string{}

	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > MaxNoteRunes {
		fields["note"] = fmt.Sprintf("Use at most %d characters.", MaxNoteRunes)
	}

	if len(in) < 1 || len(in) > MaxFiles {
		fields["files"] = fmt.Sprintf("Send between 1 and %d files.", MaxFiles)
		return Prepared{}, fields
	}

	files := make([]File, 0, len(in))
	seen := map[string]bool{}
	total := 0

	for i, f := range in {
		key := fmt.Sprintf("files[%d]", i)

		if msg := checkPath(f.Path); msg != "" {
			fields[key+".path"] = msg
			continue
		}
		lower := strings.ToLower(f.Path)
		if seen[lower] {
			fields[key+".path"] = "Duplicate file path."
			continue
		}
		seen[lower] = true

		if !utf8.ValidString(f.Content) || strings.ContainsRune(f.Content, 0) {
			fields[key+".content"] = "Only plain UTF-8 text is accepted."
			continue
		}

		total += len(f.Content)
		files = append(files, File{
			Path:     f.Path,
			Language: allowedExt[strings.ToLower(path.Ext(f.Path))],
			Content:  f.Content,
		})
	}

	if total < 1 {
		if _, exists := fields["files"]; !exists {
			fields["files"] = "The files are empty."
		}
	}
	if total > MaxTotalBytes {
		fields["files"] = "Total size must be at most 100 KB."
	}

	if len(fields) > 0 {
		return Prepared{}, fields
	}

	return Prepared{
		Files:      files,
		TotalBytes: total,
		SHA256:     hashFiles(files),
		Note:       note,
	}, nil
}

// checkPath returns an error message, or "" when the path is acceptable.
func checkPath(p string) string {
	if p == "" {
		return "Required."
	}
	if len(p) > MaxPathBytes {
		return "Path is too long."
	}
	if !utf8.ValidString(p) {
		return "Path must be valid UTF-8."
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return "Path contains a character that is not allowed."
		}
	}
	if strings.HasPrefix(p, "/") {
		return "Use a relative path."
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "Path is not valid."
		}
	}
	if _, ok := allowedExt[strings.ToLower(path.Ext(p))]; !ok {
		return "This file type is not supported."
	}
	return ""
}

// hashFiles gives the same hash no matter what order the files were sent in.
// Paths and contents cannot contain NUL bytes, so the separators are unambiguous.
func hashFiles(files []File) string {
	sorted := make([]File, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	h := sha256.New()
	for _, f := range sorted {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(f.Content))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
