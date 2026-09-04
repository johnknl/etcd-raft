package rewrite

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnknl/rewriter/internal/tooling"
)

func RewriteModulePath(repoRoot, targetModule string) (int, error) {
	changed := 0
	modPath := filepath.Join(repoRoot, "go.mod")
	b, err := os.ReadFile(modPath)
	if err != nil {
		return changed, err
	}
	lines := strings.Split(string(b), "\n")
	for i, ln := range lines {
		if strings.HasPrefix(ln, "module ") {
			n := "module " + targetModule
			if ln != n {
				lines[i] = n
				changed++
			}
			break
		}
	}
	nb := []byte(strings.Join(lines, "\n"))
	if !bytes.Equal(b, nb) {
		if err := os.WriteFile(modPath, nb, 0o644); err != nil {
			return changed, err
		}
	}

	goFiles, err := tooling.GoFiles(repoRoot)
	if err != nil {
		return changed, err
	}
	for _, rel := range goFiles {
		path := filepath.Join(repoRoot, rel)
		src, err := os.ReadFile(path)
		if err != nil {
			return changed, err
		}
		if !bytes.Contains(src, []byte("go.etcd.io/raft/v3")) {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return changed, err
		}
		localChanged := false
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, "\"")
			if !strings.HasPrefix(p, "go.etcd.io/raft/v3") {
				continue
			}
			np := strings.Replace(p, "go.etcd.io/raft/v3", targetModule, 1)
			if np != p {
				imp.Path.Value = quote(np)
				localChanged = true
			}
		}
		if !localChanged {
			continue
		}
		var out bytes.Buffer
		if err := format.Node(&out, fset, f); err != nil {
			return changed, err
		}
		if bytes.Equal(src, out.Bytes()) {
			continue
		}
		if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
			return changed, err
		}
		changed++
	}
	return changed, nil
}

func quote(s string) string { return "\"" + s + "\"" }
