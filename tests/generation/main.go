// Run from the repository root: go run ./tests/generation
// This standard-library-only harness tests the actual production generation
// declarations with minimal chunk storage, without linking graphics or workers.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := os.MkdirTemp("", "gocraft-generation-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	fset := token.NewFileSet()
	source, err := parser.ParseFile(fset, "world_gen.go", nil, 0)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	out.WriteString("package main\nimport (\"math\"; \"math/rand\")\n")
	for _, decl := range source.Decls {
		keep := false
		switch d := decl.(type) {
		case *ast.FuncDecl:
			keep = d.Recv == nil
		case *ast.GenDecl:
			keep = d.Tok == token.CONST || d.Tok == token.VAR
			// The biome value type belongs to the generation algorithm.
			if d.Tok == token.TYPE {
				for _, s := range d.Specs {
					if s.(*ast.TypeSpec).Name.Name == "BiomeParams" {
						keep = true
					}
				}
			}
		}
		if keep {
			if err := format.Node(&out, fset, decl); err != nil {
				return err
			}
			out.WriteByte('\n')
		}
	}
	types, err := parser.ParseFile(fset, "types.go", nil, 0)
	if err != nil {
		return err
	}
	for _, decl := range types.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		v, ok := g.Specs[0].(*ast.ValueSpec)
		if !ok || (v.Names[0].Name != "chunkWidth" && v.Names[0].Name != "blockAir") {
			continue
		}
		if err := format.Node(&out, fset, decl); err != nil {
			return err
		}
		out.WriteByte('\n')
	}
	out.WriteString("type Chunk struct { blocks [chunkWidth][chunkHeight][chunkWidth]byte; heightMap [chunkWidth][chunkWidth]int16 }\n")
	if err := os.WriteFile(filepath.Join(dir, "world_gen.go"), out.Bytes(), 0600); err != nil {
		return err
	}
	for _, name := range []string{"generation_terrain.go", "generation_trees.go", "generation_test.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module generationtest\ngo 1.21\n"), 0600); err != nil {
		return err
	}
	goexe := filepath.Join(os.Getenv("GOROOT"), "bin", "go")
	if os.Getenv("GOROOT") == "" {
		goexe = "go"
	}
	if _, err := os.Stat(goexe + ".exe"); err == nil {
		goexe += ".exe"
	}
	cmd := exec.Command(goexe, "test", "-count=1", "-v", ".")
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr
	return cmd.Run()
}
