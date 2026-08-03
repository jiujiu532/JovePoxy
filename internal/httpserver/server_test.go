package httpserver_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jovepoxy/internal/config"
	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/httpserver"
	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/reqlog"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

func TestServer_proxies_free_chat_with_local_bearer_key(t *testing.T) {
	// Given
	var receivedBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer public" {
			t.Errorf("upstream authorization = %q, want public auth", got)
		}
		var err error
		receivedBody, err = ioReadAll(request.Body)
		if err != nil {
			t.Errorf("read upstream request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl_1","object":"chat.completion"}`))
	}))
	defer upstream.Close()

	server := newServer(t, upstream.URL, []zen.Model{{ID: "demo-free"}})
	body := []byte(`{"model":"demo-free","messages":[{"role":"user","content":"hello"}],"tools":[{"type":"function"}],"tool_choice":{"type":"function","function":{"name":"lookup"}}}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+server.key)
	recorder := httptest.NewRecorder()

	// When
	server.handler.ServeHTTP(recorder, request)

	// Then
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !bytes.Equal(receivedBody, body) {
		t.Fatalf("upstream body = %s, want original request bytes %s", receivedBody, body)
	}
}

type testServer struct {
	handler http.Handler
	key     string
	keys    *keys.Service
	logs    *reqlog.Service
}

func newServer(t *testing.T, upstreamURL string, catalogModels []zen.Model) testServer {
	t.Helper()
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	keyService := keys.NewService(database, nil)
	created, err := keyService.Create(ctx, keys.CreateInput{Label: "proxy-test"})
	if err != nil {
		t.Fatalf("create local key: %v", err)
	}
	client, err := zen.NewClient(config.Config{ZenBase: upstreamURL, OCVersion: "test", UpstreamTimeout: time.Second})
	if err != nil {
		t.Fatalf("new Zen client: %v", err)
	}
	catalog, err := models.NewCatalog(testModelSource{models: catalogModels}, models.Settings{TTL: time.Hour})
	if err != nil {
		t.Fatalf("new catalog: %v", err)
	}
	box, err := crypto.NewBox("test-admin-secret-32-bytes-minimum!!")
	if err != nil {
		t.Fatalf("new box: %v", err)
	}
	logsService := reqlog.NewService(database, nil)
	return testServer{
		handler: httpserver.New(httpserver.Dependencies{
			Keys: keyService, Catalog: catalog, Zen: client,
			Pool: zenpool.NewService(database, box, nil), Logs: logsService,
			Version: "test-version",
		}),
		key:  created.Secret,
		keys: keyService,
		logs: logsService,
	}
}

type testModelSource struct{ models []zen.Model }

func (source testModelSource) Models(context.Context) ([]zen.Model, error) {
	return source.models, nil
}

func ioReadAll(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	return io.ReadAll(body)
}
