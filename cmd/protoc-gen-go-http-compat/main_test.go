package main

import (
	"strings"
	"testing"
)

func TestAdaptBuildPath(t *testing.T) {
	const source = `// Code generated. DO NOT EDIT.
package v1
import (
 transport "github.com/go-kratos/kratos/v3/transport/http"
 other "example.com/other"
)
const steamhttpbinding = "transport.BuildPath must remain in this string"
// transport.BuildPath must remain in this comment.
func path(in any) string {
 _ = other.BuildPath("other", in)
 return transport.BuildPath("/apps/{app_id}", in, transport.WithQueryParams())
}
`
	got, err := adaptBuildPath(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`steamhttpbinding_ "` + bindingImport + `"`,
		`steamhttpbinding_.BuildPath("/apps/{app_id}", in, transport.WithQueryParams())`,
		`other.BuildPath("other", in)`,
		`"transport.BuildPath must remain in this string"`,
		`// transport.BuildPath must remain in this comment.`,
		`// Code generated. DO NOT EDIT.`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	again, err := adaptBuildPath(got)
	if err != nil || again != got {
		t.Fatalf("not idempotent: %v", err)
	}
}

func TestAdaptBuildPathNoCalls(t *testing.T) {
	const source = "package v1\n// No HTTP client here.\n"
	got, err := adaptBuildPath(source)
	if err != nil || got != source {
		t.Fatalf("got %q, error %v", got, err)
	}
	if _, err := adaptBuildPath("invalid Go syntax"); err == nil {
		t.Fatal("expected invalid source error")
	}
}
