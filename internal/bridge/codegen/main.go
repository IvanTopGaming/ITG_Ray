// Command codegen reads internal/bridge/protocol (or a directory passed
// as a single arg) via go/packages and emits a TypeScript module that
// describes the JSON-RPC surface and event topics.
//
// Output target: ../../cmd/itgray-electron/src/shared/protocol.ts
// (relative to the package being generated for).
package main

import (
	"fmt"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

func main() {
	pkgPath := "."
	if len(os.Args) > 1 {
		pkgPath = os.Args[1]
	}
	abs, err := filepath.Abs(pkgPath)
	if err != nil {
		exit(err)
	}
	out := filepath.Join(abs, "..", "..", "..", "cmd", "itgray-electron", "src", "shared", "protocol.ts")
	if env := os.Getenv("CODEGEN_OUT"); env != "" {
		out = env
	}
	f, err := os.Create(out)
	if err != nil {
		exit(err)
	}
	defer f.Close()
	if err := generate(pkgPath, f); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, "codegen:", err)
	os.Exit(1)
}

// generate loads the Go package at pkgPath and writes the TS module to w.
// Discovery rules:
//   - Each interface ending in "Service" emits one entry per method into
//     the RpcMethods map. Method "DoThing" on "SampleService" maps to
//     "sample.doThing".
//   - Every non-interface top-level type in the package is emitted as a
//     TS interface (or alias). Additionally, any struct type referenced
//     transitively by a service method's params/result (e.g. hub.* types
//     from another package) is also emitted.
//   - Constants typed as EventTopic become a string-union "EventTopic".
func generate(pkgPath string, w io.Writer) error {
	cfg := &packages.Config{Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax | packages.NeedDeps | packages.NeedImports}
	// packages.Load treats a bare relative path (e.g. "testdata") as a
	// pattern, which only matches if it happens to be a Go module pattern.
	// Prepend "./" when the path is relative and not already prefixed.
	loadPath := pkgPath
	if !filepath.IsAbs(loadPath) && !strings.HasPrefix(loadPath, "./") && !strings.HasPrefix(loadPath, "../") && loadPath != "." {
		loadPath = "./" + loadPath
	}
	pkgs, err := packages.Load(cfg, loadPath)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 || pkgs[0].Types == nil {
		return fmt.Errorf("no Go package loaded from %s", pkgPath)
	}
	scope := pkgs[0].Types.Scope()

	type rpcEntry struct {
		Method, ParamsTS, ResultTS string
	}
	var rpcEntries []rpcEntry
	emitted := map[string]bool{}
	var typeOrder []string
	typeBodies := map[string]string{}
	var topics []string

	var emitType func(name string, t types.Type)
	emitType = func(name string, t types.Type) {
		if emitted[name] {
			return
		}
		emitted[name] = true
		typeOrder = append(typeOrder, name)
		typeBodies[name] = renderStruct(name, t)
	}

	// First pass: emit every non-interface top-level type in the package.
	// This guarantees Empty and all *Params / *Result helpers appear even
	// if no service method directly references them.
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj == nil {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		if _, isIface := named.Underlying().(*types.Interface); isIface {
			continue
		}
		// Skip the EventTopic alias itself — it's emitted as a union below.
		if name == "EventTopic" {
			continue
		}
		// Only struct types become TS interfaces here. Other named types
		// (e.g. ChainStatus = string) are skipped at top level; if they're
		// referenced by a struct field, tsTypeName resolves them to their
		// underlying name (and the field type renders as that name —
		// callers should ensure those types are emittable, or accept that
		// they appear by name without a declaration).
		if _, isStruct := named.Underlying().(*types.Struct); !isStruct {
			continue
		}
		emitType(name, named.Underlying())
	}

	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj == nil {
			continue
		}

		// Constants typed as EventTopic → topic union members.
		if cnst, ok := obj.(*types.Const); ok {
			if named, ok := cnst.Type().(*types.Named); ok && named.Obj().Name() == "EventTopic" {
				topics = append(topics, strings.Trim(cnst.Val().String(), `"`))
				continue
			}
		}

		// Service interfaces.
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		iface, ok := named.Underlying().(*types.Interface)
		if !ok {
			continue
		}
		if !strings.HasSuffix(name, "Service") {
			continue
		}
		nsName := strings.TrimSuffix(name, "Service")
		ns := strings.ToLower(nsName[:1]) + nsName[1:]
		for i := 0; i < iface.NumMethods(); i++ {
			m := iface.Method(i)
			sig := m.Type().(*types.Signature)
			if sig.Params().Len() != 1 || sig.Results().Len() != 2 {
				return fmt.Errorf("%s.%s: unexpected signature, want (Params) (Result, error)", name, m.Name())
			}
			pType := sig.Params().At(0).Type()
			rType := sig.Results().At(0).Type()
			emitNeeded(pType, emitType)
			emitNeeded(rType, emitType)
			rpcEntries = append(rpcEntries, rpcEntry{
				Method:   ns + "." + lowerFirst(m.Name()),
				ParamsTS: tsTypeName(pType),
				ResultTS: tsTypeName(rType),
			})
		}
	}
	sort.Slice(rpcEntries, func(i, j int) bool { return rpcEntries[i].Method < rpcEntries[j].Method })
	sort.Strings(topics)
	sort.Strings(typeOrder)

	fmt.Fprintln(w, "// Code generated by internal/bridge/codegen. DO NOT EDIT.")
	fmt.Fprintln(w)
	for _, n := range typeOrder {
		fmt.Fprintln(w, typeBodies[n])
	}
	fmt.Fprintln(w, "export interface RpcMethods {")
	for _, e := range rpcEntries {
		fmt.Fprintf(w, "  \"%s\": { params: %s; result: %s };\n", e.Method, e.ParamsTS, e.ResultTS)
	}
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "export type RpcMethod = keyof RpcMethods;")
	fmt.Fprintln(w, "export type RpcParams<M extends RpcMethod> = RpcMethods[M][\"params\"];")
	fmt.Fprintln(w, "export type RpcResult<M extends RpcMethod> = RpcMethods[M][\"result\"];")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "export type EventTopic =")
	for i, t := range topics {
		end := ""
		if i == len(topics)-1 {
			end = ";"
		}
		fmt.Fprintf(w, "  | \"%s\"%s\n", t, end)
	}
	return nil
}

// emitNeeded walks the given type recursively, emitting any named struct
// types (including those from other packages, e.g. hub.Snapshot →
// hub.SettingsView → hub.GeneralSettings → ...).
func emitNeeded(t types.Type, emit func(string, types.Type)) {
	switch tt := t.(type) {
	case *types.Named:
		// time.Time renders as a string at the field-type level — never
		// expand its private struct shape into the TS module.
		if tt.Obj().Pkg() != nil && tt.Obj().Pkg().Path() == "time" && tt.Obj().Name() == "Time" {
			return
		}
		st, ok := tt.Underlying().(*types.Struct)
		if !ok {
			// Could be a slice/map/string alias — descend into its
			// underlying type so e.g. type Foo []Bar still pulls in Bar.
			emitNeeded(tt.Underlying(), emit)
			return
		}
		emit(tt.Obj().Name(), st)
		for i := 0; i < st.NumFields(); i++ {
			emitNeeded(st.Field(i).Type(), emit)
		}
	case *types.Pointer:
		emitNeeded(tt.Elem(), emit)
	case *types.Slice:
		emitNeeded(tt.Elem(), emit)
	case *types.Array:
		emitNeeded(tt.Elem(), emit)
	case *types.Map:
		emitNeeded(tt.Key(), emit)
		emitNeeded(tt.Elem(), emit)
	}
}

func renderStruct(name string, t types.Type) string {
	st, ok := t.(*types.Struct)
	if !ok {
		// Named slice/map/etc — render as type alias.
		return fmt.Sprintf("export type %s = %s;", name, tsTypeName(t))
	}
	if st.NumFields() == 0 {
		return fmt.Sprintf("export interface %s {}\n", name)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "export interface %s {\n", name)
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		tag := st.Tag(i)
		jsonName, optional := jsonFieldInfo(f.Name(), tag)
		if jsonName == "-" {
			continue
		}
		colon := ":"
		if optional {
			colon = "?:"
		}
		fmt.Fprintf(&b, "  %s%s %s;\n", jsonName, colon, tsTypeName(f.Type()))
	}
	b.WriteString("}\n")
	return b.String()
}

// jsonFieldInfo returns the JSON field name and whether the field is
// optional (has the "omitempty" tag). A returned name of "-" means the
// field is excluded from JSON serialisation.
func jsonFieldInfo(goName, tag string) (string, bool) {
	if tag == "" {
		return lowerFirst(goName), false
	}
	for _, part := range strings.Split(tag, " ") {
		if strings.HasPrefix(part, "json:\"") {
			val := strings.TrimSuffix(strings.TrimPrefix(part, "json:\""), "\"")
			parts := strings.Split(val, ",")
			name := parts[0]
			optional := false
			for _, opt := range parts[1:] {
				if opt == "omitempty" {
					optional = true
				}
			}
			if name == "-" {
				return "-", false
			}
			if name == "" {
				return lowerFirst(goName), optional
			}
			return name, optional
		}
	}
	return lowerFirst(goName), false
}

func tsTypeName(t types.Type) string {
	switch tt := t.(type) {
	case *types.Basic:
		switch tt.Kind() {
		case types.Bool:
			return "boolean"
		case types.String:
			return "string"
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
			types.Float32, types.Float64:
			return "number"
		}
	case *types.Slice:
		return tsTypeName(tt.Elem()) + "[]"
	case *types.Array:
		return tsTypeName(tt.Elem()) + "[]"
	case *types.Pointer:
		return tsTypeName(tt.Elem()) + " | null"
	case *types.Named:
		// time.Time renders as a string (RFC3339 from encoding/json).
		if tt.Obj().Pkg() != nil && tt.Obj().Pkg().Path() == "time" && tt.Obj().Name() == "Time" {
			return "string"
		}
		// String aliases (e.g. ChainStatus = string) render as their
		// underlying primitive — we don't emit a TS declaration for them.
		if b, ok := tt.Underlying().(*types.Basic); ok {
			return tsTypeName(b)
		}
		return tt.Obj().Name()
	case *types.Map:
		return "Record<" + tsTypeName(tt.Key()) + ", " + tsTypeName(tt.Elem()) + ">"
	case *types.Interface:
		return "unknown"
	}
	return "unknown"
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}
