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

var getterByField = map[string]string{
	"Term":           "GetTerm",
	"Index":          "GetIndex",
	"Type":           "GetType",
	"Data":           "GetData",
	"ConfState":      "GetConfState",
	"Metadata":       "GetMetadata",
	"To":             "GetTo",
	"From":           "GetFrom",
	"LogTerm":        "GetLogTerm",
	"Commit":         "GetCommit",
	"Vote":           "GetVote",
	"Snapshot":       "GetSnapshot",
	"Reject":         "GetReject",
	"RejectHint":     "GetRejectHint",
	"Context":        "GetContext",
	"Entries":        "GetEntries",
	"Responses":      "GetResponses",
	"Voters":         "GetVoters",
	"Learners":       "GetLearners",
	"VotersOutgoing": "GetVotersOutgoing",
	"LearnersNext":   "GetLearnersNext",
	"AutoLeave":      "GetAutoLeave",
	"NodeId":         "GetNodeId",
	"Id":             "GetId",
	"Changes":        "GetChanges",
	"Transition":     "GetTransition",
}

var setterByField = map[string]string{
	"Term":           "SetTermPtr",
	"Index":          "SetIndexPtr",
	"Type":           "SetType",
	"Data":           "SetData",
	"ConfState":      "SetConfState",
	"Metadata":       "SetMetadata",
	"To":             "SetToPtr",
	"From":           "SetFromPtr",
	"LogTerm":        "SetLogTermPtr",
	"Commit":         "SetCommitPtr",
	"Vote":           "SetVotePtr",
	"Snapshot":       "SetSnapshot",
	"Reject":         "SetRejectPtr",
	"RejectHint":     "SetRejectHintPtr",
	"Context":        "SetContext",
	"Entries":        "SetEntries",
	"Responses":      "SetResponses",
	"Voters":         "SetVoters",
	"Learners":       "SetLearners",
	"VotersOutgoing": "SetVotersOutgoing",
	"LearnersNext":   "SetLearnersNext",
	"AutoLeave":      "SetAutoLeavePtr",
	"NodeId":         "SetNodeIDPtr",
	"Id":             "SetIDPtr",
	"Changes":        "SetChanges",
	"Transition":     "SetTransition",
}

func RewritePBFieldAccess(repoRoot string) (int, error) {
	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Dir: repoRoot,
		Mode: packages.NeedName | packages.NeedCompiledGoFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo,
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
			if strings.Contains(filepath.ToSlash(path), "/./") {
				continue
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return changedFiles, err
			}
			changed := false
			astutil.Apply(file, func(c *astutil.Cursor) bool {
				switch n := c.Node().(type) {
				case *ast.CompositeLit:
					named, _, ok := raftpbLitType(pkg.TypesInfo, n)
					if !ok {
						return true
					}
					alias := raftpbAliasForFile(file)
					if alias == "" {
						return true
					}
					repl, ok := rewritePBComposite(alias, named.Obj().Name(), n)
					if !ok {
						return true
					}
					c.Replace(repl)
					changed = true
					return false
				case *ast.AssignStmt:
					if n.Tok != token.ASSIGN || len(n.Lhs) != 1 || len(n.Rhs) != 1 {
						return true
					}
					sel, ok := n.Lhs[0].(*ast.SelectorExpr)
					if !ok {
						return true
					}
					field := raftpbFieldName(pkg.TypesInfo, sel)
					setter, ok := setterByField[field]
					if !ok {
						return true
					}
					call := &ast.ExprStmt{X: &ast.CallExpr{Fun: &ast.SelectorExpr{X: sel.X, Sel: ast.NewIdent(setter)}, Args: []ast.Expr{n.Rhs[0]}}}
					c.Replace(call)
					changed = true
					return false
				case *ast.CallExpr:
					if s, ok := n.Fun.(*ast.SelectorExpr); ok {
						if field := raftpbFieldName(pkg.TypesInfo, s); field != "" {
							if getter, ok := getterByField[field]; ok {
								s.Sel = ast.NewIdent(getter)
								changed = true
								return true
							}
						}
					}
					if s, ok := n.Fun.(*ast.SelectorExpr); ok {
						name := s.Sel.Name
						if strings.HasPrefix(name, "Set") && !strings.HasSuffix(name, "Ptr") && len(n.Args) == 1 {
							if isEnumArg(n.Args[0]) {
								if u := unEnumArg(n.Args[0]); u != nil {
									n.Args[0] = u
									changed = true
								}
							} else if needsPtrSetter(name) {
								s.Sel = ast.NewIdent(name + "Ptr")
								changed = true
							}
						}
					}
					return true
				case *ast.SelectorExpr:
					if isAssignLHS(c.Parent(), n) || isKVKey(c.Parent(), n) || isSelCallFun(c.Parent(), n) {
						return true
					}
					field := raftpbFieldName(pkg.TypesInfo, n)
					getter, ok := getterByField[field]
					if !ok {
						return true
					}
					c.Replace(&ast.CallExpr{Fun: &ast.SelectorExpr{X: n.X, Sel: ast.NewIdent(getter)}})
					changed = true
					return false
				}
				return true
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

func raftpbFieldName(info *types.Info, sel *ast.SelectorExpr) string {
	s := info.Selections[sel]
	if s == nil || s.Kind() != types.FieldVal {
		return ""
	}
	recv := s.Recv()
	if p, ok := recv.(*types.Pointer); ok {
		recv = p.Elem()
	}
	n, ok := recv.(*types.Named)
	if !ok || n.Obj() == nil || n.Obj().Pkg() == nil {
		return ""
	}
	if n.Obj().Pkg().Path() != "go.etcd.io/raft/v3/raftpb" {
		return ""
	}
	return s.Obj().Name()
}

func isAssignLHS(parent ast.Node, sel *ast.SelectorExpr) bool {
	a, ok := parent.(*ast.AssignStmt)
	if !ok {
		return false
	}
	for _, lhs := range a.Lhs {
		if lhs == sel {
			return true
		}
	}
	return false
}

func isKVKey(parent ast.Node, sel *ast.SelectorExpr) bool {
	kv, ok := parent.(*ast.KeyValueExpr)
	return ok && kv.Key == sel
}

func isSelCallFun(parent ast.Node, sel *ast.SelectorExpr) bool {
	c, ok := parent.(*ast.CallExpr)
	return ok && c.Fun == sel
}

func isEnumArg(e ast.Expr) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return false
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	return ok && s.Sel.Name == "Enum"
}

func unEnumArg(e ast.Expr) ast.Expr {
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != "Enum" {
		return nil
	}
	return s.X
}

func needsPtrSetter(name string) bool {
	switch name {
	case "SetTo", "SetFrom", "SetTerm", "SetLogTerm", "SetIndex", "SetCommit", "SetVote", "SetReject", "SetRejectHint", "SetNodeID", "SetID", "SetAutoLeave":
		return true
	default:
		return false
	}
}
