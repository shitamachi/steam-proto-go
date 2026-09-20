package httpbinding_test

import (
	"testing"

	"github.com/go-kratos/kratos/v3/transport/http"
	v1 "github.com/shitamachi/steam-proto-go/api/steamonline/v1"
	"github.com/shitamachi/steam-proto-go/internal/httpbinding"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestBuildPath(t *testing.T) {
	req := &v1.GetOnlinePlayersRequest{AppId: 565100, Source: v1.GetOnlinePlayersSource_GET_ONLINE_PLAYERS_SOURCE_STEAM_COMMUNITY_HTML}
	for _, tc := range []struct {
		name, pattern, want string
		msg                 any
		opts                []http.BuildPathOption
	}{
		{"snake case", "/players/{app_id}", "/players/565100?source=GET_ONLINE_PLAYERS_SOURCE_STEAM_COMMUNITY_HTML", req, []http.BuildPathOption{http.WithQueryParams()}},
		{"JSON name", "/players/{appId}", "/players/565100", req, nil},
		{"pattern suffix", "/players/{app_id=*}", "/players/565100", req, nil},
		{"omit query", "/players/{app_id}", "/players/565100", req, []http.BuildPathOption{http.WithQueryParams(), http.WithOmitFields("source")}},
		{"nil", "/players/{app_id}", "/players/{app_id}", nil, nil},
		{"typed nil", "/players/{app_id}", "/players/{app_id}", (*v1.GetOnlinePlayersRequest)(nil), nil},
		{"query only", "/players", "/players?appId=565100&source=GET_ONLINE_PLAYERS_SOURCE_STEAM_COMMUNITY_HTML", req, []http.BuildPathOption{http.WithQueryParams()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := httpbinding.BuildPath(tc.pattern, tc.msg, tc.opts...); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	if req.AppId != 565100 || req.Source != v1.GetOnlinePlayersSource_GET_ONLINE_PLAYERS_SOURCE_STEAM_COMMUNITY_HTML {
		t.Fatal("path construction changed the request")
	}
}

func TestBuildPathNestedCustomJSONName(t *testing.T) {
	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name: proto.String("path_test.proto"), Syntax: proto.String("proto3"), Package: proto.String("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Child"), Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("app_id"), JsonName: proto.String("applicationID"), Number: proto.Int32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
			}},
			{Name: proto.String("Request"), Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("app_info"), JsonName: proto.String("appInfo"), Number: proto.Int32(1), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".test.Child")},
				{Name: proto.String("page_size"), JsonName: proto.String("pageSize"), Number: proto.Int32(2), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
			}},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	child := dynamicpb.NewMessage(fd.Messages().ByName("Child"))
	child.Set(child.Descriptor().Fields().ByName("app_id"), protoreflect.ValueOfInt32(214340))
	req := dynamicpb.NewMessage(fd.Messages().ByName("Request"))
	req.Set(req.Descriptor().Fields().ByName("app_info"), protoreflect.ValueOfMessage(child))
	req.Set(req.Descriptor().Fields().ByName("page_size"), protoreflect.ValueOfInt32(20))
	for _, pattern := range []string{"/apps/{app_info.app_id}", "/apps/{appInfo.applicationID}"} {
		if got := httpbinding.BuildPath(pattern, req, http.WithQueryParams()); got != "/apps/214340?pageSize=20" {
			t.Fatalf("%s: %s", pattern, got)
		}
	}
}
