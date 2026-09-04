package tooling

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func PrivatizePBFields(repoRoot string) error {
	root := filepath.Join(repoRoot, "raftpb")
	fieldMap, err := fieldRenameMap(filepath.Join(root, "raft.pb.go"))
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return err
		}

		changed := false
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeSpec:
				s, ok := x.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, fld := range s.Fields.List {
					if len(fld.Names) != 1 {
						continue
					}
					old := fld.Names[0].Name
					nw, ok := fieldMap[old]
					if !ok || old == nw {
						continue
					}
					fld.Names[0].Name = nw
					changed = true
				}
			case *ast.SelectorExpr:
				if nw, ok := fieldMap[x.Sel.Name]; ok && x.Sel.Name != nw {
					x.Sel.Name = nw
					changed = true
				}
			case *ast.KeyValueExpr:
				k, ok := x.Key.(*ast.Ident)
				if !ok {
					return true
				}
				if nw, ok := fieldMap[k.Name]; ok && k.Name != nw {
					k.Name = nw
					changed = true
				}
			}
			return true
		})

		if !changed {
			continue
		}
		var out bytes.Buffer
		if err := format.Node(&out, fset, f); err != nil {
			return fmt.Errorf("format %s: %w", path, err)
		}
		if _, err := WriteIfChanged(path, out.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func fieldRenameMap(pbPath string) (map[string]string, error) {
	src, err := os.ReadFile(pbPath)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, pbPath, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	fieldMap := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		s, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, fld := range s.Fields.List {
			if len(fld.Names) != 1 {
				continue
			}
			name := fld.Names[0].Name
			if !ast.IsExported(name) {
				continue
			}
			if name == "state" || name == "unknownFields" || name == "sizeCache" {
				continue
			}
			priv := strings.ToLower(name[:1]) + name[1:]
			if name == "Type" {
				priv = "type_"
			}
			fieldMap[name] = priv
		}
		return true
	})
	return fieldMap, nil
}
