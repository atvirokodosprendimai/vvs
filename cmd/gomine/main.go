// gomine — chunk Go source files by AST symbol boundaries and emit JSONL.
// Each output line is one Chunk (func, method, type, interface, const group).
// Pipe into scripts/mine-go.py to import into ChromaDB.
//
// Usage:
//
//	gomine [paths...]          # one or more files or directories; defaults to .
//	gomine ./internal ./cmd    # multiple directories
//	gomine internal/app/app.go internal/app/builder.go  # specific files
package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Chunk is one addressable symbol extracted from a Go source file.
type Chunk struct {
	File      string `json:"file"`
	Package   string `json:"package"`
	Symbol    string `json:"symbol"`
	Kind      string `json:"kind"`     // func | method | type | interface | const | var
	Receiver  string `json:"receiver"` // non-empty for methods
	Signature string `json:"signature"`
	Doc       string `json:"doc"`
	Body      string `json:"body"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

func main() {
	enc := json.NewEncoder(os.Stdout)

	paths := os.Args[1:]
	if len(paths) == 0 {
		paths = []string{"."}
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			log.Printf("skip %s: %v", path, err)
			continue
		}
		if info.IsDir() {
			if err := walkDir(enc, path); err != nil {
				log.Printf("walk %s: %v", path, err)
			}
		} else if strings.HasSuffix(path, ".go") {
			emitFile(enc, path)
		}
	}
}

func walkDir(enc *json.Encoder, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base != "." && (strings.HasPrefix(base, ".") || base == "vendor" || base == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		emitFile(enc, path)
		return nil
	})
}

func emitFile(enc *json.Encoder, path string) {
	chunks, err := parseFile(path)
	if err != nil {
		log.Printf("skip %s: %v", path, err)
		return
	}
	for _, c := range chunks {
		if err := enc.Encode(c); err != nil {
			log.Printf("encode %s: %v", path, err)
			return
		}
	}
}

func parseFile(path string) ([]Chunk, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		// Partial parse still useful — continue with what we got
		if f == nil {
			return nil, err
		}
	}

	pkg := f.Name.Name
	var chunks []Chunk

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			chunks = append(chunks, funcChunk(fset, src, path, pkg, d))
		case *ast.GenDecl:
			chunks = append(chunks, genChunks(fset, src, path, pkg, d)...)
		}
	}
	return chunks, nil
}

func funcChunk(fset *token.FileSet, src []byte, path, pkg string, d *ast.FuncDecl) Chunk {
	start := fset.Position(d.Pos())
	end := fset.Position(d.End())
	body := slice(src, d.Pos(), d.End(), fset)

	kind := "func"
	receiver := ""
	if d.Recv != nil && len(d.Recv.List) > 0 {
		kind = "method"
		receiver = recvType(d.Recv.List[0])
	}

	sig := signature(src, d, fset)
	doc := docText(d.Doc)

	return Chunk{
		File:      path,
		Package:   pkg,
		Symbol:    d.Name.Name,
		Kind:      kind,
		Receiver:  receiver,
		Signature: sig,
		Doc:       doc,
		Body:      body,
		StartLine: start.Line,
		EndLine:   end.Line,
	}
}

func genChunks(fset *token.FileSet, src []byte, path, pkg string, d *ast.GenDecl) []Chunk {
	var chunks []Chunk
	switch d.Tok {
	case token.TYPE:
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			kind := "type"
			if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
				kind = "interface"
			}
			start := fset.Position(d.Pos())
			end := fset.Position(d.End())
			body := slice(src, d.Pos(), d.End(), fset)
			doc := docText(d.Doc)
			if ts.Comment != nil && doc == "" {
				doc = docText(ts.Comment)
			}
			chunks = append(chunks, Chunk{
				File:      path,
				Package:   pkg,
				Symbol:    ts.Name.Name,
				Kind:      kind,
				Signature: "type " + ts.Name.Name,
				Doc:       doc,
				Body:      body,
				StartLine: start.Line,
				EndLine:   end.Line,
			})
		}
	case token.CONST, token.VAR:
		if len(d.Specs) == 0 {
			break
		}
		start := fset.Position(d.Pos())
		end := fset.Position(d.End())
		body := slice(src, d.Pos(), d.End(), fset)
		kind := "const"
		if d.Tok == token.VAR {
			kind = "var"
		}
		symbol := ""
		if vs, ok := d.Specs[0].(*ast.ValueSpec); ok && len(vs.Names) > 0 {
			symbol = vs.Names[0].Name
			if len(d.Specs) > 1 {
				symbol += "…"
			}
		}
		chunks = append(chunks, Chunk{
			File:      path,
			Package:   pkg,
			Symbol:    symbol,
			Kind:      kind,
			Doc:       docText(d.Doc),
			Body:      body,
			StartLine: start.Line,
			EndLine:   end.Line,
		})
	}
	return chunks
}

func slice(src []byte, start, end token.Pos, fset *token.FileSet) string {
	s := fset.Position(start).Offset
	e := fset.Position(end).Offset
	if s < 0 || e > len(src) || s > e {
		return ""
	}
	return string(src[s:e])
}

func signature(src []byte, d *ast.FuncDecl, fset *token.FileSet) string {
	startOff := fset.Position(d.Pos()).Offset
	var endOff int
	if d.Body != nil {
		endOff = fset.Position(d.Body.Lbrace).Offset
	} else {
		endOff = fset.Position(d.End()).Offset
	}
	if startOff < 0 || endOff > len(src) || startOff > endOff {
		return d.Name.Name
	}
	return strings.TrimSpace(string(src[startOff:endOff]))
}

func recvType(field *ast.Field) string {
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return "*" + id.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func docText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return strings.TrimSpace(cg.Text())
}
