package rewrite

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

func RewriteFallbackPrivatePrep(paths []string) (int, error) {
	changed := 0
	for _, p := range paths {
		ok, err := rewriteFallbackFile(p)
		if err != nil {
			return changed, err
		}
		if ok {
			changed++
		}
	}
	return changed, nil
}

func rewriteFallbackFile(path string) (bool, error) {
	sp := filepath.ToSlash(path)
	if strings.Contains(sp, "/raftpb/") || strings.Contains(sp, "/./") {
		return false, nil
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if !bytes.Contains(src, []byte("pb.")) && !bytes.Contains(src, []byte("raftpb.")) {
		return false, nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return false, err
	}
	alias := raftpbAliasForFile(f)
	if alias == "" {
		return false, nil
	}

	changed := false
	astutil.Apply(f, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.CallExpr:
			if s, ok := n.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "SetType" && len(n.Args) == 1 {
				if u := unEnumArg(n.Args[0]); u != nil {
					n.Args[0] = u
					changed = true
				}
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				if ce, ok := n.X.(*ast.CallExpr); ok {
					if s, ok := ce.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "GetData" && len(ce.Args) == 0 {
						c.Replace(&ast.CallExpr{Fun: &ast.SelectorExpr{X: s.X, Sel: ast.NewIdent("DataRef")}})
						changed = true
						return false
					}
				}
			}
		case *ast.SelectorExpr:
			if isAssignLHS(c.Parent(), n) || isKVKey(c.Parent(), n) || isSelCallFun(c.Parent(), n) {
				return true
			}
			if n.Sel.Name == "Metadata" && !metadataSelectorContext(c.Parent()) && !likelyPBExpr(n.X) {
				return true
			}
			if n.Sel.Name == "ConfState" && !confStateSelectorContext(c.Parent()) && !likelyPBExpr(n.X) {
				return true
			}
			if getter, ok := map[string]string{
				"Metadata":  "GetMetadata",
				"ConfState": "GetConfState",
			}[n.Sel.Name]; ok {
				c.Replace(&ast.CallExpr{Fun: &ast.SelectorExpr{X: n.X, Sel: ast.NewIdent(getter)}})
				changed = true
				return false
			}
		case *ast.CompositeLit:
			if n.Type != nil {
				return true
			}
			fields := keyedFields(n)
			if len(fields) == 0 {
				return true
			}
			if _, ok := fields["Data"]; ok {
				repl := rewriteEntry(alias, fields)
				c.Replace(repl)
				changed = true
				return false
			}
		}
		return true
	}, nil)

	var outSrc []byte
	if changed {
		var out bytes.Buffer
		if err := format.Node(&out, fset, f); err != nil {
			return false, err
		}
		outSrc = out.Bytes()
	} else {
		outSrc = src
	}

	// Last-resort textual cleanup for selector chains that are hard to
	// rewrite safely in mixed-type contexts.
	ns := strings.ReplaceAll(string(outSrc), ".Metadata.", ".GetMetadata().")
	if ns == string(src) {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(ns), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func likelyPBExpr(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		n := strings.ToLower(x.Name)
		return n == "cs" || n == "cc" || n == "s" || strings.Contains(n, "snap") || strings.Contains(n, "conf")
	case *ast.SelectorExpr:
		n := strings.ToLower(x.Sel.Name)
		if strings.Contains(n, "snap") || strings.Contains(n, "conf") || n == "metadata" {
			return true
		}
		return likelyPBExpr(x.X)
	case *ast.CallExpr:
		if s, ok := x.Fun.(*ast.SelectorExpr); ok {
			n := strings.ToLower(s.Sel.Name)
			if strings.HasPrefix(n, "get") && (strings.Contains(n, "snap") || strings.Contains(n, "conf") || strings.Contains(n, "metadata")) {
				return true
			}
		}
		return likelyPBExpr(x.Fun)
	default:
		return false
	}
}

func metadataSelectorContext(parent ast.Node) bool {
	s, ok := parent.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch s.Sel.Name {
	case "SetIndex", "SetIndexPtr", "SetTerm", "SetTermPtr", "SetConfState", "GetConfState", "SetVoters", "SetLearners", "SetVotersOutgoing", "SetLearnersNext", "GetIndex", "GetTerm":
		return true
	default:
		return false
	}
}

func confStateSelectorContext(parent ast.Node) bool {
	s, ok := parent.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch s.Sel.Name {
	case "SetVoters", "SetLearners", "SetVotersOutgoing", "SetLearnersNext", "GetVoters", "GetLearners", "GetVotersOutgoing", "GetLearnersNext":
		return true
	default:
		return false
	}
}
