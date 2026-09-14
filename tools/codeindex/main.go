// Command codeindex maintains the root/platform production symbol inventory.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	write := flag.Bool("write", false, "regenerate CODE_SYMBOLS.md")
	check := flag.Bool("check", false, "fail if CODE_SYMBOLS.md is stale")
	flag.Parse()
	if err := run(*write, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(write, check bool) error {
	if write && check {
		return fmt.Errorf("choose -write or -check, not both")
	}
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("run from repository root: %w", err)
	}
	var files []string
	for _, dir := range []string{".", "platform"} {
		paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			return err
		}
		for _, path := range paths {
			if !strings.HasSuffix(path, "_test.go") {
				files = append(files, filepath.ToSlash(path))
			}
		}
	}
	sort.Strings(files)
	var out bytes.Buffer
	out.WriteString("# 文件与符号索引（自动生成）\n\n先按功能查找：[CODE_INDEX.md](CODE_INDEX.md)。本表覆盖根目录与 platform 的非测试 Go 文件。\n\n更新：`go run ./tools/codeindex -write`；检查：`go run ./tools/codeindex -check`。不要手工编辑本文件。\n\n")
	for _, path := range files {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		fmt.Fprintf(&out, "## [%s](%s)\n\n", path, path)
		var types, functions []string
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok == token.TYPE {
					for _, spec := range d.Specs {
						types = append(types, spec.(*ast.TypeSpec).Name.Name)
					}
				}
			case *ast.FuncDecl:
				name := d.Name.Name
				if d.Recv != nil {
					name = receiverName(d.Recv.List[0].Type) + "." + name
				}
				functions = append(functions, name)
			}
		}
		if len(types) > 0 {
			fmt.Fprintf(&out, "类型：`%s`\n\n", strings.Join(types, "`、`"))
		}
		if len(functions) > 0 {
			fmt.Fprintf(&out, "函数/方法：`%s`\n\n", strings.Join(functions, "`、`"))
		}
		if len(types) == 0 && len(functions) == 0 {
			out.WriteString("常量或包级数据定义。\n\n")
		}
	}
	if check {
		old, err := os.ReadFile("CODE_SYMBOLS.md")
		if err != nil {
			return err
		}
		if !bytes.Equal(bytes.ReplaceAll(old, []byte("\r\n"), []byte("\n")), out.Bytes()) {
			return fmt.Errorf("CODE_SYMBOLS.md is stale; run go run ./tools/codeindex -write")
		}
		return nil
	}
	if write {
		return os.WriteFile("CODE_SYMBOLS.md", out.Bytes(), 0644)
	}
	_, err := os.Stdout.Write(out.Bytes())
	return err
}

func receiverName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + receiverName(e.X)
	case *ast.IndexExpr:
		return receiverName(e.X)
	}
	return "receiver"
}
