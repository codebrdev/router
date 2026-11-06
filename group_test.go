package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codebrdev/middleware"
)

func testMiddleware(headerValue string) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-Header", headerValue)
			next.ServeHTTP(w, r)
		})
	}
}

func TestRouteGroup(t *testing.T) {

	// 1. Scenario: Create a new route group with a prefix "" and middleware
	test := []struct {
		name             string
		prefix           string
		groupMiddlewares []middleware.Middleware // Middlewares applied to the entire route group
		routeMiddlewares []middleware.Middleware // Middlewares applied to the specific route within the group
		method           string
		path             string
		expectedStatus   int
		expectedHeader   string
	}{
		{
			name:             "Empty prefix with middleware",
			prefix:           "",
			groupMiddlewares: nil,
			routeMiddlewares: nil,
			method:           "GET",
			path:             "/",
			expectedStatus:   http.StatusOK,
			expectedHeader:   "",
		},
		{
			name:             "Non-empty prefix with middleware",
			prefix:           "/api/v1",
			groupMiddlewares: nil,
			routeMiddlewares: nil,
			method:           "GET",
			path:             "/api",
			expectedStatus:   http.StatusOK,
			expectedHeader:   "",
		},
		{
			name:             "Non-empty prefix with route middleware",
			prefix:           "/api/v1",
			groupMiddlewares: []middleware.Middleware{testMiddleware("middleware-group")},
			routeMiddlewares: nil,
			method:           "GET",
			path:             "/protected",
			expectedStatus:   http.StatusOK,
			expectedHeader:   "middleware-group",
		},
		{
			name:             "Non-empty prefix with both group and route middleware",
			prefix:           "/api/v1",
			groupMiddlewares: []middleware.Middleware{testMiddleware("middleware-group")},
			routeMiddlewares: []middleware.Middleware{testMiddleware("middleware-route")},
			method:           "GET",
			path:             "/protected",
			expectedStatus:   http.StatusOK,
			expectedHeader:   "middleware-route",
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()

			gm := tt.groupMiddlewares
			if gm == nil {
				gm = append(gm, func(next http.Handler) http.Handler {
					return next
				})
			}

			group := NewRouteGroup(mux, tt.prefix, gm...)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			// Register the handler for the route with the RouteGroup
			if tt.routeMiddlewares != nil {
				group.Handle(tt.method, tt.path, handler, tt.routeMiddlewares...)
			} else {
				group.Handle(tt.method, tt.path, handler)
			}

			fullURL := tt.prefix + tt.path
			req := httptest.NewRequest(tt.method, fullURL, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			resp := rec.Result()
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if tt.expectedHeader != "" {
				headerVal := rec.Header().Get("X-Test-Header")
				if headerVal != tt.expectedHeader {
					t.Errorf("expected header %q, got %q", tt.expectedHeader, headerVal)
				}
			}
		})

	}

}

func TestRouteGroupRootPathMatchesQueryStringOnlyAtRoot(t *testing.T) {
	mux := http.NewServeMux()
	group := NewRouteGroup(mux, "")
	group.HandleFunc("GET", "/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.Query().Get("page")))
	})

	rootRequest := httptest.NewRequest(http.MethodGet, "/?page=2", nil)
	rootResponse := httptest.NewRecorder()
	mux.ServeHTTP(rootResponse, rootRequest)

	if rootResponse.Code != http.StatusOK {
		t.Fatalf("expected root request status %d, got %d", http.StatusOK, rootResponse.Code)
	}
	if rootResponse.Body.String() != "2" {
		t.Fatalf("expected query parameter to be available as %q, got %q", "2", rootResponse.Body.String())
	}

	subpathRequest := httptest.NewRequest(http.MethodGet, "/nested?page=2", nil)
	subpathResponse := httptest.NewRecorder()
	mux.ServeHTTP(subpathResponse, subpathRequest)

	if subpathResponse.Code != http.StatusNotFound {
		t.Errorf("expected subpath request status %d, got %d", http.StatusNotFound, subpathResponse.Code)
	}
}

// Com capacidade sobrando no slice do grupo, um append direto faria as rotas compartilharem o mesmo array
func TestRouteGroupRoutesDoNotShareMiddlewares(t *testing.T) {
	mark := func(name string) middleware.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("X-Order", name)
				next.ServeHTTP(w, r)
			})
		}
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	groupMiddlewares := make([]middleware.Middleware, 1, 8)
	groupMiddlewares[0] = mark("group")

	mux := http.NewServeMux()
	rg := NewRouteGroup(mux, "/g", groupMiddlewares...)
	rg.Handle("GET", "/a", ok, mark("a"))
	rg.Handle("GET", "/b", ok, mark("b"))

	for path, want := range map[string]string{"/g/a": "group,a", "/g/b": "group,b"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := strings.Join(rec.Header().Values("X-Order"), ","); got != want {
			t.Errorf("%s: middlewares = %q, esperado %q", path, got, want)
		}
	}
}
