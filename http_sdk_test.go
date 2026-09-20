package steamproto_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	kratohttp "github.com/go-kratos/kratos/v3/transport/http"
	db "github.com/shitamachi/steam-proto-go/api/steamdb/v1"
	online "github.com/shitamachi/steam-proto-go/api/steamonline/v1"
	review "github.com/shitamachi/steam-proto-go/api/steamreview/v1"
	todo "github.com/shitamachi/steam-proto-go/api/todo/v1"
)

// Exercise the generated clients, including transport encoding and query/body
// separation. Testing only the path helper would miss a generator regression.
func TestGeneratedHTTPClientRequests(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		query              url.Values
		body               bool
		call               func(context.Context, *kratohttp.Client) error
	}{
		{"online", "GET", "/v1/steamonline/players/565100", url.Values{"source": {"GET_ONLINE_PLAYERS_SOURCE_STEAM_COMMUNITY_HTML"}}, false,
			func(ctx context.Context, c *kratohttp.Client) error {
				_, err := online.NewSteamOnlineServiceHTTPClient(c).GetOnlinePlayers(ctx, &online.GetOnlinePlayersRequest{AppId: 565100, Source: online.GetOnlinePlayersSource_GET_ONLINE_PLAYERS_SOURCE_STEAM_COMMUNITY_HTML})
				return err
			}},
		{"reviews", "GET", "/v1/steamreview/pages/214340", url.Values{"cursor": {"a+/=& b"}, "numPerPage": {"20"}}, false,
			func(ctx context.Context, c *kratohttp.Client) error {
				_, err := review.NewSteamReviewServiceHTTPClient(c).FetchReviewsPage(ctx, &review.FetchReviewsPageRequest{AppId: 214340, Cursor: "a+/=& b", NumPerPage: 20})
				return err
			}},
		{"steamdb", "GET", "/v1/steamdb/apps/570", url.Values{"cc": {"cn"}, "subpages": {"charts", "depots"}}, false,
			func(ctx context.Context, c *kratohttp.Client) error {
				_, err := db.NewSteamDBServiceHTTPClient(c).GetAppPage(ctx, &db.GetAppPageRequest{AppId: 570, Cc: "cn", Subpages: []string{"charts", "depots"}})
				return err
			}},
		{"unchanged simple path", "GET", "/v1/todos/42", url.Values{}, false,
			func(ctx context.Context, c *kratohttp.Client) error {
				_, err := todo.NewTodoServiceHTTPClient(c).GetTodo(ctx, &todo.GetTodoRequest{Id: 42})
				return err
			}},
		{"query only", "GET", "/v1/todos/list", url.Values{"pageSize": {"20"}, "pageToken": {"next token"}}, false,
			func(ctx context.Context, c *kratohttp.Client) error {
				_, err := todo.NewTodoServiceHTTPClient(c).ListTodos(ctx, &todo.ListTodosRequest{PageSize: 20, PageToken: "next token"})
				return err
			}},
		{"body excluded from query", "POST", "/v1/todos/create", url.Values{}, true,
			func(ctx context.Context, c *kratohttp.Client) error {
				_, err := todo.NewTodoServiceHTTPClient(c).CreateTodo(ctx, &todo.CreateTodoRequest{Todo: &todo.Todo{Title: "test"}})
				return err
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan struct{}, 1)
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- struct{}{}
				if r.Method != tc.method || r.URL.Path != tc.path || !reflect.DeepEqual(r.URL.Query(), tc.query) {
					t.Errorf("got %s %s; want %s %s, query %v", r.Method, r.URL, tc.method, tc.path, tc.query)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || (len(body) > 0) != tc.body {
					t.Errorf("body %q, error %v", body, err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("{}"))
			}))
			defer s.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			c, err := kratohttp.NewClient(ctx, kratohttp.WithEndpoint(s.URL))
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.call(ctx, c); err != nil {
				t.Fatal(err)
			}
			select {
			case <-requests:
			default:
				t.Fatal("client did not issue an HTTP request")
			}
		})
	}
}
