// protoc-gen-go-http-compat delegates generation to the pinned upstream plugin,
// then routes generated BuildPath calls through our protobuf-aware adapter.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

const upstream = "github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3@v3.0.0-20260526000039-30da04b769dc"
const bindingImport = "github.com/shitamachi/steam-proto-go/internal/httpbinding"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	request, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "run", upstream)
	cmd.Stdin = bytes.NewReader(request)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("upstream HTTP generator: %w", err)
	}
	response := &pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(output, response); err != nil {
		return err
	}
	if response.GetError() == "" {
		for _, file := range response.File {
			if !strings.HasSuffix(file.GetName(), "_http.pb.go") {
				continue
			}
			content, err := adaptBuildPath(file.GetContent())
			if err != nil {
				return fmt.Errorf("%s: %w", file.GetName(), err)
			}
			file.Content = proto.String(content)
		}
	}
	output, err = proto.Marshal(response)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(output)
	return err
}

func adaptBuildPath(content string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generated.go", content, parser.ParseComments)
	if err != nil {
		return "", err
	}
	httpAlias := ""
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return "", err
		}
		if path == "github.com/go-kratos/kratos/v3/transport/http" {
			httpAlias = "http"
			if imp.Name != nil {
				httpAlias = imp.Name.Name
			}
		}
	}
	used := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
	alias := "steamhttpbinding"
	for used[alias] {
		alias += "_"
	}
	changed := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "BuildPath" {
			return true
		}
		id, ok := selector.X.(*ast.Ident)
		if ok && id.Name == httpAlias {
			selector.X = ast.NewIdent(alias)
			changed = true
		}
		return true
	})
	if !changed {
		return content, nil
	}
	imp := &ast.ImportSpec{Name: ast.NewIdent(alias), Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(bindingImport)}}
	for _, decl := range file.Decls {
		if group, ok := decl.(*ast.GenDecl); ok && group.Tok == token.IMPORT {
			group.Specs = append(group.Specs, imp)
			break
		}
	}
	var out bytes.Buffer
	if err := format.Node(&out, fset, file); err != nil {
		return "", err
	}
	return out.String(), nil
}
