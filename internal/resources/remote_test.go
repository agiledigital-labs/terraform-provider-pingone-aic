package resources

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/client"
	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/testutil"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Helper-level tests of remoteName cannot see a write path that never calls it —
// that is how an Update after a prefix change orphaned a tree on 2026-08-14.
// Drive each name-keyed resource's real write through a fake transport and
// assert the request target.
func TestWriteTargetsPersistedNameAfterPrefixChange(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
		avoid  string
		seed   []string
		write  func(*testing.T, *client.Client)
	}{
		{
			name: "journey update", prefix: "NewPrefix_",
			want: "OldPrefix_X", avoid: "NewPrefix_X",
			write: writeJourney(false),
		},
		{
			name: "journey create", prefix: "NewPrefix_",
			want:  "NewPrefix_X",
			write: writeJourney(true),
		},
		{
			name: "oauth2 client update", prefix: "NewPrefix_",
			want: "OldPrefix_X", avoid: "NewPrefix_X",
			write: writeOAuth2(false),
		},
		{
			name: "oauth2 client create", prefix: "NewPrefix_",
			want:  "NewPrefix_X",
			write: writeOAuth2(true),
		},
		{
			name: "esv variable update", prefix: "NewPrefix_",
			want: "esv-oldprefix-x", avoid: "esv-newprefix-x",
			write: writeESVVariable(false),
		},
		{
			name: "esv variable create", prefix: "NewPrefix_",
			want:  "esv-newprefix-x",
			write: writeESVVariable(true),
		},
		{
			name: "esv secret update", prefix: "NewPrefix_",
			want: "esv-oldprefix-x", avoid: "esv-newprefix-x",
			write: writeESVSecret(false),
		},
		{
			name: "esv secret create", prefix: "NewPrefix_",
			want:  "esv-newprefix-x",
			write: writeESVSecret(true),
		},
		{
			name: "managed object update", prefix: "NewPrefix_",
			want: "OldPrefix_X", avoid: "NewPrefix_X",
			write: writeManaged(false),
		},
		{
			name: "managed object create", prefix: "NewPrefix_",
			want:  "NewPrefix_X",
			write: writeManaged(true),
		},
		{
			name: "idm endpoint update", prefix: "NewPrefix_",
			want: "OldPrefix_X", avoid: "NewPrefix_X",
			write: writeEndpoint(false),
		},
		{
			name: "idm endpoint create", prefix: "NewPrefix_",
			want:  "NewPrefix_X",
			write: writeEndpoint(true),
		},
		{
			name: "idm schedule update", prefix: "NewPrefix_",
			want: "OldPrefix_X", avoid: "NewPrefix_X",
			write: writeSchedule(false),
		},
		{
			name: "idm schedule create", prefix: "NewPrefix_",
			want:  "NewPrefix_X",
			write: writeSchedule(true),
		},
		{
			name: "internal role update", prefix: "NewPrefix_",
			want: "OldPrefix_X", avoid: "NewPrefix_X",
			seed:  []string{"/openidm/internal/role/OldPrefix_X"},
			write: writeRole(false),
		},
		{
			name: "internal role create", prefix: "NewPrefix_",
			want:  "NewPrefix_X",
			write: writeRole(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits []string
			exists := map[string]bool{}
			for _, p := range tt.seed {
				exists[p] = true
			}
			var managed []byte

			httpClient := testutil.Client(func(req *http.Request) (*http.Response, error) {
				var body []byte
				if req.Body != nil {
					body, _ = io.ReadAll(req.Body)
					_ = req.Body.Close()
				}
				if req.Method == http.MethodPut || req.Method == http.MethodPost {
					hits = append(hits, writeTarget(req, body))
				}
				status, resp := fakeWriteResponse(req, body, exists, &managed)
				return &http.Response{
					StatusCode: status,
					Body:       io.NopCloser(strings.NewReader(resp)),
					Header:     make(http.Header),
				}, nil
			})
			c, err := client.New(client.Config{
				TenantURL: "https://tenant.example", AccessToken: "token",
				ResourcePrefix: tt.prefix, HTTPClient: httpClient,
			})
			if err != nil {
				t.Fatal(err)
			}
			tt.write(t, c)
			if len(hits) == 0 {
				t.Fatal("write issued no PUT/POST")
			}
			for _, hit := range hits {
				if !strings.Contains(hit, tt.want) {
					t.Fatalf("write went to %q, want it to address %q", hit, tt.want)
				}
				if tt.avoid != "" && strings.Contains(hit, tt.avoid) {
					t.Fatalf("write went to %q — that creates a second object and orphans %s", hit, tt.want)
				}
			}
		})
	}
}

func writeTarget(req *http.Request, body []byte) string {
	// Managed config is a whole-document RMW: the name lives in objects[],
	// not in the URL.
	if req.URL.Path == "/openidm/config/managed" {
		var doc struct {
			Objects []struct {
				Name string `json:"name"`
			} `json:"objects"`
		}
		_ = json.Unmarshal(body, &doc)
		names := make([]string, 0, len(doc.Objects))
		for _, o := range doc.Objects {
			names = append(names, o.Name)
		}
		return strings.Join(names, ",")
	}
	return req.URL.Path
}

func fakeWriteResponse(req *http.Request, body []byte, exists map[string]bool, managed *[]byte) (int, string) {
	path := req.URL.Path
	id := path[strings.LastIndex(path, "/")+1:]
	switch {
	case strings.Contains(path, "/trees/"):
		return http.StatusOK, `{"_id":"` + id + `","enabled":true,"entryNodeId":"entry",` +
			`"identityResource":"managed/alpha_user","nodes":{},"staticNodes":{},"uiConfig":{}}`
	case strings.Contains(path, "/OAuth2Client/"):
		return http.StatusOK, `{"_id":"` + id + `"}`
	case strings.Contains(path, "/environment/variables/"):
		return http.StatusOK, `{"_id":"` + id + `","expressionType":"string","valueBase64":"","loaded":false}`
	case strings.Contains(path, "/environment/secrets/"):
		if req.Method == http.MethodPost && !strings.Contains(path, "/versions") {
			return http.StatusOK, ""
		}
		if strings.HasSuffix(path, "/versions") {
			id = path[strings.LastIndex(strings.TrimSuffix(path, "/versions"), "/")+1:]
		}
		return http.StatusOK, `{"_id":"` + id + `","encoding":"generic","useInPlaceholders":true}`
	case strings.Contains(path, "/openidm/config/endpoint/"):
		return http.StatusOK, `{"_id":"endpoint/` + id + `","type":"text/javascript","source":"return {};"}`
	case strings.Contains(path, "/openidm/config/schedule/"):
		return http.StatusOK, `{"_id":"schedule/` + id + `","enabled":false,"persisted":true,"type":"cron","invokeService":"script"}`
	case strings.Contains(path, "/openidm/internal/role/"):
		if req.Method == http.MethodPut {
			exists[path] = true
			return http.StatusOK, `{"_id":"` + id + `","_rev":"1","name":"` + id + `","privileges":[]}`
		}
		if !exists[path] {
			return http.StatusNotFound, `{}`
		}
		return http.StatusOK, `{"_id":"` + id + `","_rev":"1","name":"` + id + `","privileges":[]}`
	case path == "/openidm/config/managed":
		if req.Method == http.MethodPut {
			*managed = append([]byte(nil), body...)
			return http.StatusOK, string(body)
		}
		if len(*managed) == 0 {
			return http.StatusOK, `{"_id":"managed","objects":[]}`
		}
		return http.StatusOK, string(*managed)
	default:
		return http.StatusNotFound, `{}`
	}
}

func writeCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

func writeJourney(create bool) func(*testing.T, *client.Client) {
	return func(t *testing.T, c *client.Client) {
		t.Helper()
		ctx, cancel := writeCtx()
		defer cancel()
		plan := journeyModel{
			Name: types.StringValue("X"), Realm: types.StringValue("alpha"),
			Enabled: types.BoolValue(true), InnerTreeOnly: types.BoolValue(false),
			MustRun: types.BoolValue(false), NoSession: types.BoolValue(false),
			TransactionalOnly: types.BoolValue(false),
			EntryNode:         types.StringValue("entry"),
			Categories:        types.ListNull(types.StringType),
		}
		prior := journeyModel{}
		if !create {
			prior = journeyModel{
				ID: types.StringValue("OldPrefix_X"), RemoteName: types.StringValue("OldPrefix_X"),
				Name: types.StringValue("X"), Realm: types.StringValue("alpha"),
			}
		}
		if _, err := (&journeyResource{client: c}).write(ctx, plan, prior); err != nil {
			t.Fatal(err)
		}
	}
}

func writeOAuth2(create bool) func(*testing.T, *client.Client) {
	return func(t *testing.T, c *client.Client) {
		t.Helper()
		ctx, cancel := writeCtx()
		defer cancel()
		priorID, priorRemote := types.StringNull(), types.StringNull()
		if !create {
			priorID = types.StringValue("OldPrefix_X")
			priorRemote = types.StringValue("OldPrefix_X")
		}
		if _, _, _, _, err := (&oauth2ClientResource{client: c}).write(ctx, priorID, priorRemote, map[string]any{
			"_realm": "alpha", "_name": "X",
		}, ""); err != nil {
			t.Fatal(err)
		}
	}
}

func writeESVVariable(create bool) func(*testing.T, *client.Client) {
	return func(t *testing.T, c *client.Client) {
		t.Helper()
		ctx, cancel := writeCtx()
		defer cancel()
		plan := esvVariableModel{
			Name: types.StringValue("esv-x"), ExpressionType: types.StringValue("string"),
			Value: types.StringValue("v"),
		}
		prior := esvVariableModel{}
		if !create {
			prior = esvVariableModel{
				ID: types.StringValue("esv-oldprefix-x"), RemoteName: types.StringValue("esv-oldprefix-x"),
				Name: types.StringValue("esv-x"),
			}
		}
		if _, err := (&esvVariableResource{client: c}).write(ctx, plan, prior); err != nil {
			t.Fatal(err)
		}
	}
}

func writeESVSecret(create bool) func(*testing.T, *client.Client) {
	return func(t *testing.T, c *client.Client) {
		t.Helper()
		ctx, cancel := writeCtx()
		defer cancel()
		plan := esvSecretModel{
			Name: types.StringValue("esv-x"), Encoding: types.StringValue("generic"),
			UseInPlaceholders: types.BoolValue(true), Value: types.StringValue("s"),
			Description: types.StringValue("new"),
		}
		prior := esvSecretModel{}
		if !create {
			prior = esvSecretModel{
				ID: types.StringValue("esv-oldprefix-x"), RemoteName: types.StringValue("esv-oldprefix-x"),
				Name: types.StringValue("esv-x"), Encoding: types.StringValue("generic"),
				UseInPlaceholders: types.BoolValue(true), Value: types.StringValue("s"),
				Description: types.StringValue("old"),
			}
		}
		if _, err := (&esvSecretResource{client: c}).write(ctx, plan, prior); err != nil {
			t.Fatal(err)
		}
	}
}

func writeManaged(create bool) func(*testing.T, *client.Client) {
	return func(t *testing.T, c *client.Client) {
		t.Helper()
		ctx, cancel := writeCtx()
		defer cancel()
		plan := managedObjectModel{Name: types.StringValue("X"), Title: types.StringValue("X")}
		prior := managedObjectModel{}
		if !create {
			prior = managedObjectModel{
				ID: types.StringValue("OldPrefix_X"), RemoteName: types.StringValue("OldPrefix_X"),
				Name: types.StringValue("X"),
			}
		}
		if _, err := (&managedObjectResource{client: c}).write(ctx, plan, prior); err != nil {
			t.Fatal(err)
		}
	}
}

func writeEndpoint(create bool) func(*testing.T, *client.Client) {
	return func(t *testing.T, c *client.Client) {
		t.Helper()
		ctx, cancel := writeCtx()
		defer cancel()
		plan := idmEndpointModel{
			Name: types.StringValue("X"), Type: types.StringValue("text/javascript"),
			Source: types.StringValue("return {};"),
		}
		prior := idmEndpointModel{}
		if !create {
			prior = idmEndpointModel{
				ID: types.StringValue("OldPrefix_X"), RemoteName: types.StringValue("OldPrefix_X"),
				Name: types.StringValue("X"),
			}
		}
		if _, err := (&idmEndpointResource{client: c}).write(ctx, plan, prior); err != nil {
			t.Fatal(err)
		}
	}
}

func writeSchedule(create bool) func(*testing.T, *client.Client) {
	return func(t *testing.T, c *client.Client) {
		t.Helper()
		ctx, cancel := writeCtx()
		defer cancel()
		plan := idmScheduleModel{
			Name: types.StringValue("X"), Type: types.StringValue("cron"),
			InvokeService: types.StringValue("script"), Enabled: types.BoolValue(false),
			Persisted: types.BoolValue(true), ScriptType: types.StringValue("text/javascript"),
		}
		prior := idmScheduleModel{}
		if !create {
			prior = idmScheduleModel{
				ID: types.StringValue("OldPrefix_X"), RemoteName: types.StringValue("OldPrefix_X"),
				Name: types.StringValue("X"),
			}
		}
		if _, err := (&idmScheduleResource{client: c}).write(ctx, plan, prior); err != nil {
			t.Fatal(err)
		}
	}
}

func writeRole(create bool) func(*testing.T, *client.Client) {
	return func(t *testing.T, c *client.Client) {
		t.Helper()
		ctx, cancel := writeCtx()
		defer cancel()
		plan := internalRoleModel{Name: types.StringValue("X")}
		prior := internalRoleModel{}
		if !create {
			prior = internalRoleModel{
				ID: types.StringValue("OldPrefix_X"), RemoteName: types.StringValue("OldPrefix_X"),
				Name: types.StringValue("X"),
			}
		}
		if _, err := (&internalRoleResource{client: c}).write(ctx, plan, prior); err != nil {
			t.Fatal(err)
		}
	}
}
