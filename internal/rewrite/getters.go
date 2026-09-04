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
)

var confStateGetters = map[string]string{
	"Voters":         "GetVoters",
	"Learners":       "GetLearners",
	"VotersOutgoing": "GetVotersOutgoing",
	"LearnersNext":   "GetLearnersNext",
	"AutoLeave":      "GetAutoLeave",
}

func RewriteConfStateSelectorsToGetters(paths []string) (int, error) {
	changed := 0
	for _, p := range paths {
		ok, err := rewriteConfStateSelectorsFile(p)
		if err != nil {
			return changed, err
		}
		if ok {
			changed++
		}
	}
	return changed, nil
}

func rewriteConfStateSelectorsFile(path string) (bool, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	sp := filepath.ToSlash(path)
	if strings.Contains(sp, "/raftpb/") || strings.HasPrefix(sp, "raftpb/") {
		return false, nil
	}
	if !bytes.Contains(src, []byte("ConfState")) && !bytes.Contains(src, []byte("Voters")) {
		return false, nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return false, err
	}
	if len(raftpbAliases(f)) == 0 {
		return false, nil
	}

	changed := false
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range x.Rhs {
				nr := rewriteGetterExpr(rhs)
				if nr != rhs {
					x.Rhs[i] = nr
					changed = true
				}
			}
		case *ast.ValueSpec:
			for i, v := range x.Values {
				nv := rewriteGetterExpr(v)
				if nv != v {
					x.Values[i] = nv
					changed = true
				}
			}
		case *ast.ReturnStmt:
			for i, r := range x.Results {
				nr := rewriteGetterExpr(r)
				if nr != r {
					x.Results[i] = nr
					changed = true
				}
			}
		case *ast.CallExpr:
			for i, a := range x.Args {
				na := rewriteGetterExpr(a)
				if na != a {
					x.Args[i] = na
					changed = true
				}
			}
			nf := rewriteGetterExpr(x.Fun)
			if nf != x.Fun {
				x.Fun = nf
				changed = true
			}
		case *ast.SendStmt:
			nv := rewriteGetterExpr(x.Value)
			if nv != x.Value {
				x.Value = nv
				changed = true
			}
		case *ast.ExprStmt:
			nx := rewriteGetterExpr(x.X)
			if nx != x.X {
				x.X = nx
				changed = true
			}
		case *ast.KeyValueExpr:
			nv := rewriteGetterExpr(x.Value)
			if nv != x.Value {
				x.Value = nv
				changed = true
			}
		case *ast.RangeStmt:
			nx := rewriteGetterExpr(x.X)
			if nx != x.X {
				x.X = nx
				changed = true
			}
		}
		return true
	})

	if !changed {
		return false, nil
	}
	var out bytes.Buffer
	if err := format.Node(&out, fset, f); err != nil {
		return false, err
	}
	if bytes.Equal(src, out.Bytes()) {
		return false, nil
	}
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func rewriteGetterExpr(e ast.Expr) ast.Expr {
	switch x := e.(type) {
	case *ast.ParenExpr:
		nx := rewriteGetterExpr(x.X)
		if nx != x.X {
			return &ast.ParenExpr{X: nx}
		}
		return e
	case *ast.CallExpr:
		ch := false
		args := make([]ast.Expr, len(x.Args))
		copy(args, x.Args)
		for i, a := range args {
			na := rewriteGetterExpr(a)
			if na != a {
				args[i] = na
				ch = true
			}
		}
		nf := rewriteGetterExpr(x.Fun)
		if nf != x.Fun {
			ch = true
		}
		if !ch {
			return e
		}
		nc := *x
		nc.Args = args
		nc.Fun = nf
		return &nc
	case *ast.SliceExpr:
		ch := false
		nx := rewriteGetterExpr(x.X)
		if nx != x.X {
			x.X = nx
			ch = true
		}
		if x.Low != nil {
			nl := rewriteGetterExpr(x.Low)
			if nl != x.Low {
				x.Low = nl
				ch = true
			}
		}
		if x.High != nil {
			nh := rewriteGetterExpr(x.High)
			if nh != x.High {
				x.High = nh
				ch = true
			}
		}
		if x.Max != nil {
			nm := rewriteGetterExpr(x.Max)
			if nm != x.Max {
				x.Max = nm
				ch = true
			}
		}
		if ch {
			return x
		}
		return e
	case *ast.IndexExpr:
		ch := false
		nx := rewriteGetterExpr(x.X)
		if nx != x.X {
			x.X = nx
			ch = true
		}
		ni := rewriteGetterExpr(x.Index)
		if ni != x.Index {
			x.Index = ni
			ch = true
		}
		if ch {
			return x
		}
		return e
	case *ast.UnaryExpr:
		nx := rewriteGetterExpr(x.X)
		if nx != x.X {
			x.X = nx
			return x
		}
		return e
	case *ast.SelectorExpr:
		nx := rewriteGetterExpr(x.X)
		if getter, ok := confStateGetters[x.Sel.Name]; ok {
			if !isLikelyConfStateExpr(nx) {
				if nx != x.X {
					return &ast.SelectorExpr{X: nx, Sel: x.Sel}
				}
				return e
			}
			if strings.HasPrefix(x.Sel.Name, "Get") {
				if nx != x.X {
					return &ast.SelectorExpr{X: nx, Sel: x.Sel}
				}
				return e
			}
			return &ast.CallExpr{Fun: &ast.SelectorExpr{X: nx, Sel: ident(getter)}}
		}
		if nx != x.X {
			return &ast.SelectorExpr{X: nx, Sel: x.Sel}
		}
		return e
	default:
		return e
	}
}

func isLikelyConfStateExpr(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		n := strings.ToLower(x.Name)
		return n == "cs" || strings.Contains(n, "confstate") || strings.HasSuffix(n, "cs")
	case *ast.SelectorExpr:
		n := strings.ToLower(x.Sel.Name)
		if strings.Contains(n, "confstate") {
			return true
		}
		return isLikelyConfStateExpr(x.X)
	case *ast.CallExpr:
		if s, ok := x.Fun.(*ast.SelectorExpr); ok {
			n := strings.ToLower(s.Sel.Name)
			if n == "getconfstate" || strings.Contains(n, "confstate") {
				return true
			}
		}
		return isLikelyConfStateExpr(x.Fun)
	case *ast.ParenExpr:
		return isLikelyConfStateExpr(x.X)
	case *ast.StarExpr:
		return isLikelyConfStateExpr(x.X)
	default:
		return false
	}
}
