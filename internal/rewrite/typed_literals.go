package rewrite

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

func RewriteTypedPBLiterals(repoRoot string) (int, error) {
	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Dir:   repoRoot,
		Mode:  packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Fset:  fset,
		Tests: true,
	}, "./...")
	if err != nil {
		return 0, err
	}
	changedFiles := 0
	for _, pkg := range pkgs {
		if strings.HasSuffix(pkg.PkgPath, "/raftpb") {
			continue
		}
		for _, file := range pkg.Syntax {
			path := fset.Position(file.Pos()).Filename
			if path == "" {
				continue
			}
			sp := filepath.ToSlash(path)
			if strings.Contains(sp, "/./") {
				continue
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return changedFiles, err
			}
			alias := raftpbAliasForFile(file)
			if alias == "" {
				continue
			}
			changed := false
			astutil.Apply(file, func(c *astutil.Cursor) bool {
				cl, ok := c.Node().(*ast.CompositeLit)
				if !ok {
					return true
				}
				named, ptr, ok := raftpbLitType(pkg.TypesInfo, cl)
				if !ok {
					return true
				}
				repl, ok := rewritePBComposite(alias, named.Obj().Name(), cl)
				if !ok {
					return true
				}
				if !ptr {
					repl = &ast.StarExpr{X: repl}
				}
				c.Replace(repl)
				changed = true
				return false
			}, nil)

			if !changed {
				continue
			}
			var out bytes.Buffer
			if err := format.Node(&out, fset, file); err != nil {
				return changedFiles, err
			}
			if bytes.Equal(src, out.Bytes()) {
				continue
			}
			if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
				return changedFiles, err
			}
			changedFiles++
		}
	}
	return changedFiles, nil
}

func raftpbAliasForFile(f *ast.File) string {
	aliases := raftpbAliases(f)
	if len(aliases) == 0 {
		return ""
	}
	if aliases["pb"] {
		return "pb"
	}
	if aliases["raftpb"] {
		return "raftpb"
	}
	for a := range aliases {
		if a != "_" && a != "." {
			return a
		}
	}
	return ""
}

func raftpbLitType(info *types.Info, cl *ast.CompositeLit) (*types.Named, bool, bool) {
	t := info.TypeOf(cl)
	if t == nil {
		return nil, false, false
	}
	ptr := false
	if p, ok := t.(*types.Pointer); ok {
		ptr = true
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok || n.Obj() == nil || n.Obj().Pkg() == nil {
		return nil, false, false
	}
	if !strings.HasSuffix(n.Obj().Pkg().Path(), "/raftpb") {
		return nil, false, false
	}
	s := n.Obj().Name()
	switch s {
	case "Message", "Entry", "HardState", "ConfChange", "ConfChangeSingle", "ConfChangeV2", "ConfState", "Snapshot", "SnapshotMetadata":
		return n, ptr, true
	default:
		return nil, false, false
	}
}

func rewritePBComposite(alias, structName string, cl *ast.CompositeLit) (ast.Expr, bool) {
	f := keyedFields(cl)
	if len(f) == 0 {
		switch structName {
		case "Message":
			return ctor(alias, "NewEmptyMessage"), true
		case "Entry":
			return ctor(alias, "NewEmptyEntry"), true
		case "HardState":
			return ctor(alias, "NewEmptyHardState"), true
		case "ConfChange":
			return ctor(alias, "NewEmptyConfChange"), true
		case "ConfChangeSingle":
			return ctor(alias, "NewEmptyConfChangeSingle"), true
		case "ConfChangeV2":
			return ctor(alias, "NewEmptyConfChangeV2"), true
		case "ConfState":
			return ctor(alias, "NewEmptyConfState"), true
		case "SnapshotMetadata":
			return ctor(alias, "NewEmptySnapshotMetadata"), true
		case "Snapshot":
			return ctor(alias, "NewEmptySnapshot"), true
		}
	}
	var e ast.Expr
	switch structName {
	case "Message":
		e = rewriteMessage(alias, f)
	case "Entry":
		e = rewriteEntry(alias, f)
	case "HardState":
		e = rewriteHardState(alias, f)
	case "ConfChange":
		e = rewriteConfChange(alias, f)
	case "ConfChangeSingle":
		e = rewriteConfChangeSingle(alias, f)
	case "ConfChangeV2":
		e = rewriteConfChangeV2(alias, f)
	case "ConfState":
		e = rewriteConfState(alias, f)
	case "SnapshotMetadata":
		e = rewriteSnapshotMetadata(alias, f)
	case "Snapshot":
		e = rewriteSnapshot(alias, f)
	default:
		return nil, false
	}
	return e, true
}
