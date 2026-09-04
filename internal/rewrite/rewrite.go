package rewrite

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func RewriteFile(path string) (bool, error) {
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
	aliases := raftpbAliases(f)
	if len(aliases) == 0 {
		return false, nil
	}

	changed := false
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		switch x := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range x.Rhs {
				nrhs := rewriteExpr(rhs, aliases)
				if nrhs != rhs {
					x.Rhs[i] = nrhs
					changed = true
				}
			}
		case *ast.ValueSpec:
			for i, v := range x.Values {
				nv := rewriteExpr(v, aliases)
				if nv != v {
					x.Values[i] = nv
					changed = true
				}
			}
		case *ast.ReturnStmt:
			for i, r := range x.Results {
				nr := rewriteExpr(r, aliases)
				if nr != r {
					x.Results[i] = nr
					changed = true
				}
			}
		case *ast.CallExpr:
			for i, a := range x.Args {
				na := rewriteExpr(a, aliases)
				if na != a {
					x.Args[i] = na
					changed = true
				}
			}
			nf := rewriteExpr(x.Fun, aliases)
			if nf != x.Fun {
				x.Fun = nf
				changed = true
			}
		case *ast.SendStmt:
			nv := rewriteExpr(x.Value, aliases)
			if nv != x.Value {
				x.Value = nv
				changed = true
			}
		case *ast.ExprStmt:
			nx := rewriteExpr(x.X, aliases)
			if nx != x.X {
				x.X = nx
				changed = true
			}
		case *ast.CompositeLit:
			for i, elt := range x.Elts {
				nelt := rewriteEltExpr(elt, aliases)
				if nelt != elt {
					x.Elts[i] = nelt
					changed = true
				}
			}
		case *ast.KeyValueExpr:
			nv := rewriteExpr(x.Value, aliases)
			if nv != x.Value {
				x.Value = nv
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

func RewriteFiles(paths []string) (int, error) {
	changed := 0
	for _, p := range paths {
		ok, err := RewriteFile(p)
		if err != nil {
			return changed, err
		}
		if ok {
			changed++
		}
	}
	return changed, nil
}

func raftpbAliases(f *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		if !strings.HasSuffix(path, "/raftpb") {
			continue
		}
		if imp.Name != nil {
			out[imp.Name.Name] = true
			continue
		}
		out[filepath.Base(path)] = true
	}
	return out
}

func rewriteEltExpr(e ast.Expr, aliases map[string]bool) ast.Expr {
	if kv, ok := e.(*ast.KeyValueExpr); ok {
		nv := rewriteExpr(kv.Value, aliases)
		if nv != kv.Value {
			return &ast.KeyValueExpr{Key: kv.Key, Value: nv}
		}
	}
	return rewriteExpr(e, aliases)
}

func rewriteExpr(e ast.Expr, aliases map[string]bool) ast.Expr {
	if p, ok := e.(*ast.ParenExpr); ok {
		nx := rewriteExpr(p.X, aliases)
		if nx != p.X {
			return &ast.ParenExpr{X: nx}
		}
		return e
	}
	if c, ok := e.(*ast.CallExpr); ok {
		ch := false
		args := make([]ast.Expr, len(c.Args))
		copy(args, c.Args)
		for i, a := range c.Args {
			na := rewriteExpr(a, aliases)
			if na != a {
				args[i] = na
				ch = true
			}
		}
		nf := rewriteExpr(c.Fun, aliases)
		if nf != c.Fun {
			ch = true
		}
		if s, ok := c.Fun.(*ast.SelectorExpr); ok && len(args) == 1 {
			if scalarSetter(s.Sel.Name) {
				orig := args[0]
				if narg, ok := scalarMaybe(stripEnum(orig)); ok {
					if narg != orig {
						args[0] = narg
						ch = true
					}
				} else if !isEnumCall(orig) {
					if ptr := ptrSetter(s.Sel.Name); ptr != "" && s.Sel.Name != ptr {
						nf = &ast.SelectorExpr{X: s.X, Sel: ident(ptr)}
						ch = true
					}
				}
			}
		}
		if s, ok := c.Fun.(*ast.SelectorExpr); ok {
			for _, i := range ctorScalarArgIndexes(s.Sel.Name) {
				if i < 0 || i >= len(args) {
					continue
				}
				orig := args[i]
				if narg, ok := scalarMaybe(stripEnum(orig)); ok && narg != orig {
					args[i] = narg
					ch = true
				}
			}
		}
		if ch {
			nc := *c
			nc.Fun = nf
			nc.Args = args
			return &nc
		}
		return e
	}
	if s, ok := e.(*ast.SelectorExpr); ok {
		nx := rewriteExpr(s.X, aliases)
		if nx != s.X {
			return &ast.SelectorExpr{X: nx, Sel: s.Sel}
		}
		return e
	}

	u, ok := e.(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return e
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok {
		return e
	}
	sel, ok := cl.Type.(*ast.SelectorExpr)
	if !ok {
		return e
	}
	alias, ok := sel.X.(*ast.Ident)
	if !ok || !aliases[alias.Name] {
		return e
	}

	fields := keyedFields(cl)
	switch sel.Sel.Name {
	case "Message":
		return rewriteMessage(alias.Name, fields)
	case "Entry":
		return rewriteEntry(alias.Name, fields)
	case "HardState":
		return rewriteHardState(alias.Name, fields)
	case "ConfChange":
		return rewriteConfChange(alias.Name, fields)
	case "ConfChangeSingle":
		return rewriteConfChangeSingle(alias.Name, fields)
	case "ConfChangeV2":
		return rewriteConfChangeV2(alias.Name, fields)
	case "ConfState":
		return rewriteConfState(alias.Name, fields)
	case "SnapshotMetadata":
		return rewriteSnapshotMetadata(alias.Name, fields)
	case "Snapshot":
		return rewriteSnapshot(alias.Name, fields)
	default:
		return e
	}
}

func keyedFields(cl *ast.CompositeLit) map[string]ast.Expr {
	out := map[string]ast.Expr{}
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		k, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		out[k.Name] = kv.Value
	}
	return out
}

func rewriteMessage(alias string, f map[string]ast.Expr) ast.Expr {
	typ, hasType := f["Type"]
	from, hasFrom := f["From"]
	to, hasTo := f["To"]
	var base ast.Expr
	if hasType && hasFrom && hasTo {
		if fsv, okF := scalarMaybe(from); okF {
			if tsv, okT := scalarMaybe(to); okT {
				base = ctor(alias, "NewMessage", stripEnum(typ), fsv, tsv)
				delete(f, "Type")
				delete(f, "From")
				delete(f, "To")
			}
		}
	} else if hasType {
		base = ctor(alias, "NewMessage", stripEnum(typ), intLit(0), intLit(0))
		delete(f, "Type")
	} else {
		base = ctor(alias, "NewEmptyMessage")
	}
	if base == nil {
		base = ctor(alias, "NewEmptyMessage")
	}
	return applySetters(base, f, map[string]string{
		"Type":       "SetType",
		"To":         "SetTo",
		"From":       "SetFrom",
		"Term":       "SetTerm",
		"LogTerm":    "SetLogTerm",
		"Index":      "SetIndex",
		"Commit":     "SetCommit",
		"Vote":       "SetVote",
		"Snapshot":   "SetSnapshot",
		"Reject":     "SetReject",
		"RejectHint": "SetRejectHint",
		"Context":    "SetContext",
		"Entries":    "SetEntries",
		"Responses":  "SetResponses",
	})
}

func rewriteEntry(alias string, f map[string]ast.Expr) ast.Expr {
	if len(f) == 0 {
		return ctor(alias, "NewEmptyEntry")
	}
	if only(f, "Data") {
		return ctor(alias, "NewEntryData", f["Data"])
	}
	if only(f, "Term", "Index") {
		if t, okT := scalarMaybe(f["Term"]); okT {
			if i, okI := scalarMaybe(f["Index"]); okI {
				return ctor(alias, "NewEntryRef", t, i)
			}
		}
	}
	base := ctor(alias, "NewEmptyEntry")
	return applySetters(base, f, map[string]string{"Term": "SetTerm", "Index": "SetIndex", "Type": "SetType", "Data": "SetData"})
}

func rewriteHardState(alias string, f map[string]ast.Expr) ast.Expr {
	if len(f) == 0 {
		return ctor(alias, "NewEmptyHardState")
	}
	if only(f, "Term", "Vote", "Commit") {
		if t, okT := scalarMaybe(f["Term"]); okT {
			if v, okV := scalarMaybe(f["Vote"]); okV {
				if c, okC := scalarMaybe(f["Commit"]); okC {
					return ctor(alias, "NewHardState", t, v, c)
				}
			}
		}
	}
	base := ctor(alias, "NewEmptyHardState")
	return applySetters(base, f, map[string]string{"Term": "SetTerm", "Vote": "SetVote", "Commit": "SetCommit"})
}

func rewriteConfChange(alias string, f map[string]ast.Expr) ast.Expr {
	typ, hasType := f["Type"]
	nid, hasNid := f["NodeId"]
	ctx, hasCtx := f["Context"]
	var base ast.Expr
	if hasType && hasNid {
		if !hasCtx {
			ctx = ident("nil")
		}
		if n, ok := scalarMaybe(nid); ok {
			base = ctor(alias, "NewConfChange", stripEnum(typ), n, ctx)
			delete(f, "Type")
			delete(f, "NodeId")
			delete(f, "Context")
		}
	} else {
		base = ctor(alias, "NewEmptyConfChange")
	}
	if base == nil {
		base = ctor(alias, "NewEmptyConfChange")
	}
	return applySetters(base, f, map[string]string{"Type": "SetType", "NodeId": "SetNodeID", "Context": "SetContext", "Id": "SetID"})
}

func rewriteConfChangeSingle(alias string, f map[string]ast.Expr) ast.Expr {
	typ, hasType := f["Type"]
	nid, hasNid := f["NodeId"]
	if hasType && hasNid {
		if n, ok := scalarMaybe(nid); ok {
			delete(f, "Type")
			delete(f, "NodeId")
			return applySetters(ctor(alias, "NewConfChangeSingle", stripEnum(typ), n), f, map[string]string{"Type": "SetType", "NodeId": "SetNodeID"})
		}
	}
	return applySetters(ctor(alias, "NewEmptyConfChangeSingle"), f, map[string]string{"Type": "SetType", "NodeId": "SetNodeID"})
}

func rewriteConfChangeV2(alias string, f map[string]ast.Expr) ast.Expr {
	base := ctor(alias, "NewEmptyConfChangeV2")
	return applySetters(base, f, map[string]string{"Transition": "SetTransition", "Changes": "SetChanges", "Context": "SetContext"})
}

func rewriteConfState(alias string, f map[string]ast.Expr) ast.Expr {
	base := ctor(alias, "NewEmptyConfState")
	return applySetters(base, f, map[string]string{"Voters": "SetVoters", "Learners": "SetLearners", "VotersOutgoing": "SetVotersOutgoing", "LearnersNext": "SetLearnersNext", "AutoLeave": "SetAutoLeave"})
}

func rewriteSnapshotMetadata(alias string, f map[string]ast.Expr) ast.Expr {
	base := ctor(alias, "NewEmptySnapshotMetadata")
	return applySetters(base, f, map[string]string{"ConfState": "SetConfState", "Index": "SetIndex", "Term": "SetTerm"})
}

func rewriteSnapshot(alias string, f map[string]ast.Expr) ast.Expr {
	base := ctor(alias, "NewEmptySnapshot")
	return applySetters(base, f, map[string]string{"Metadata": "SetMetadata", "Data": "SetData"})
}

func applySetters(base ast.Expr, f map[string]ast.Expr, setters map[string]string) ast.Expr {
	order := []string{"Type", "From", "To", "Term", "LogTerm", "Index", "Commit", "Vote", "Snapshot", "Reject", "RejectHint", "Context", "Entries", "Responses", "NodeId", "Id", "Transition", "Changes", "Voters", "Learners", "VotersOutgoing", "LearnersNext", "AutoLeave", "ConfState", "Metadata", "Data"}
	seen := map[string]bool{}
	for _, key := range order {
		seen[key] = true
		setter, ok := setters[key]
		if !ok {
			continue
		}
		v, ok := f[key]
		if !ok {
			continue
		}
		arg := stripEnum(v)
		useSetter := setter
		if scalarField(key) {
			if sv, ok := scalarMaybe(arg); ok {
				arg = sv
			} else if !isEnumCall(v) {
				if ps := ptrSetter(setter); ps != "" {
					useSetter = ps
					arg = v
				}
			}
		}
		base = &ast.CallExpr{Fun: selExpr(base, useSetter), Args: []ast.Expr{arg}}
	}
	for key, v := range f {
		if seen[key] {
			continue
		}
		if setter, ok := setters[key]; ok {
			arg := stripEnum(v)
			useSetter := setter
			if scalarField(key) {
				if sv, ok := scalarMaybe(arg); ok {
					arg = sv
				} else if !isEnumCall(v) {
					if ps := ptrSetter(setter); ps != "" {
						useSetter = ps
						arg = v
					}
				}
			}
			base = &ast.CallExpr{Fun: selExpr(base, useSetter), Args: []ast.Expr{arg}}
		}
	}
	return base
}

func scalarField(key string) bool {
	switch key {
	case "To", "From", "Term", "LogTerm", "Index", "Commit", "Vote", "Reject", "RejectHint", "NodeId", "Id", "AutoLeave":
		return true
	default:
		return false
	}
}

func scalarSetter(name string) bool {
	switch name {
	case "SetTo", "SetFrom", "SetTerm", "SetLogTerm", "SetIndex", "SetCommit", "SetVote", "SetReject", "SetRejectHint", "SetNodeID", "SetID", "SetAutoLeave":
		return true
	default:
		return false
	}
}

func ptrSetter(name string) string {
	if !scalarSetter(name) {
		return ""
	}
	return name + "Ptr"
}

func ctorScalarArgIndexes(name string) []int {
	switch name {
	case "NewMessage":
		return []int{0, 1, 2}
	case "NewEntryRef":
		return []int{0, 1}
	case "NewHardState":
		return []int{0, 1, 2}
	case "NewConfChange":
		return []int{0, 1}
	case "NewConfChangeSingle":
		return []int{0, 1}
	case "NewConfChangeV2":
		return []int{0}
	case "NewSnapshotMetadata":
		return []int{0, 1}
	default:
		return nil
	}
}

func only(m map[string]ast.Expr, keys ...string) bool {
	if len(m) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

func stripEnum(e ast.Expr) ast.Expr {
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return e
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Enum" {
		return e
	}
	return sel.X
}

func scalarMaybe(e ast.Expr) (ast.Expr, bool) {
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return e, false
	}
	id, ok := c.Fun.(*ast.Ident)
	if !ok || id.Name != "new" {
		return e, false
	}
	return c.Args[0], true
}

func isEnumCall(e ast.Expr) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return false
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "Enum"
}

func ctor(alias, name string, args ...ast.Expr) ast.Expr {
	return &ast.CallExpr{Fun: sel(alias, name), Args: args}
}

func sel(alias, name string) ast.Expr {
	return &ast.SelectorExpr{X: ident(alias), Sel: ident(name)}
}

func selExpr(x ast.Expr, name string) ast.Expr {
	return &ast.SelectorExpr{X: x, Sel: ident(name)}
}

func ident(name string) *ast.Ident { return ast.NewIdent(name) }

func intLit(v int) ast.Expr { return &ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", v)} }
