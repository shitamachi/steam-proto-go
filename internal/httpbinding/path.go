// Package httpbinding adapts protobuf path fields to Kratos' form encoding.
package httpbinding

import (
	"regexp"
	"strings"

	"github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var pathVariable = regexp.MustCompile(`\{([.\w]+)(=[^{}]*)?\}`)

// BuildPath keeps the wire contract unchanged while resolving template field
// names with the same keys as Kratos v3.0.0's protobuf form encoder.
// In particular, {app_id} must read and remove the encoded appId query value.
func BuildPath(pattern string, msg any, opts ...http.BuildPathOption) string {
	if m, ok := msg.(proto.Message); ok && m != nil && m.ProtoReflect().IsValid() {
		pattern = pathVariable.ReplaceAllStringFunc(pattern, func(variable string) string {
			parts := pathVariable.FindStringSubmatch(variable)
			fields := strings.Split(parts[1], ".")
			desc := m.ProtoReflect().Descriptor()
			for i, name := range fields {
				field := desc.Fields().ByName(protoreflect.Name(name))
				if field == nil {
					field = desc.Fields().ByJSONName(name)
				}
				if field == nil {
					return variable
				}
				fields[i] = field.TextName()
				if field.HasJSONName() {
					fields[i] = field.JSONName()
				}
				if i < len(fields)-1 {
					if field.Message() == nil || field.IsList() || field.IsMap() {
						return variable
					}
					desc = field.Message()
				}
			}
			return "{" + strings.Join(fields, ".") + parts[2] + "}"
		})
	}
	return http.BuildPath(pattern, msg, opts...)
}
