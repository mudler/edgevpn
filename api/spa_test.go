package api

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func newUIEcho(t *testing.T) *echo.Echo {
	t.Helper()
	ec := echo.New()
	if err := registerUI(ec); err != nil {
		t.Fatalf("registerUI: %v", err)
	}
	return ec
}

func TestRootRedirectsToApp(t *testing.T) {
	ec := newUIEcho(t)
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("got %d, want 301", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/app" {
		t.Fatalf("Location = %q, want /app", got)
	}
}

func TestAppServesIndexWithNoCache(t *testing.T) {
	ec := newUIEcho(t)
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", cc)
	}
}

func TestDeepLinkServesIndex(t *testing.T) {
	ec := newUIEcho(t)
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/nodes", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("deep link got %d, want 200", rec.Code)
	}
}

func TestMissingAssetReturns404(t *testing.T) {
	ec := newUIEcho(t)
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/does-not-exist.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing asset got %d, want 404", rec.Code)
	}
}

func TestRealAssetIsImmutablyCached(t *testing.T) {
	// Asset filenames are content-hashed by Vite and therefore unknowable
	// at authoring time, so discover one from the embedded bundle.
	entries, err := fs.ReadDir(reactUI, "react-ui/dist/assets")
	if err != nil || len(entries) == 0 {
		t.Skip("no built assets embedded; run 'make react-ui-force' first")
	}
	ec := newUIEcho(t)
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/"+entries[0].Name(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("asset %s got %d, want 200", entries[0].Name(), rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != immutableAssetCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", cc, immutableAssetCacheControl)
	}
}

// TestIndexReferencesAbsoluteAssets guards the Vite `base` setting. With a
// relative base ('./') the index emits "./assets/index-<hash>.js", which a
// browser sitting on /app/nodes resolves to /app/assets/index-<hash>.js. That
// path is swallowed by the /app/* SPA route and answered with index.html as
// text/html, so the module is rejected and the page renders blank — a failure
// invisible to every handler-level test here.
func TestIndexReferencesAbsoluteAssets(t *testing.T) {
	index, err := fs.ReadFile(reactUI, "react-ui/dist/index.html")
	if err != nil {
		t.Skip("no built index embedded; run 'make react-ui-force' first")
	}
	body := string(index)
	if !strings.Contains(body, `"/assets/`) {
		t.Fatalf("index.html has no absolute /assets/ reference:\n%s", body)
	}
	if strings.Contains(body, `"./assets/`) {
		t.Fatal(`index.html uses a relative "./assets/" base; deep links will serve HTML for the JS module`)
	}
}

func TestUnknownAPIPathReturnsJSONNotIndex(t *testing.T) {
	ec := newUIEcho(t)
	req := httptest.NewRequest(http.MethodGet, "/api/nope", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	ec.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "text/html; charset=utf-8" {
		t.Fatal("API 404 was swallowed by the SPA fallback")
	}
}

// TestUnknownAPIPathWithHTMLAcceptStillReturnsJSON exercises the path-prefix
// guard specifically. TestUnknownAPIPathReturnsJSONNotIndex sends
// Accept: application/json, which the Accept guard alone already rejects, so
// it would pass even if the /api prefix check were deleted. Browsers, proxies
// and fetch() calls that inherit the document's Accept header all send
// text/html; if the fallback fired for those, an API 404 would come back as a
// 200 HTML page and the frontend's get() helper — which assumes any 2xx body
// is JSON — would raise a confusing SyntaxError instead of a clean error.
func TestUnknownAPIPathWithHTMLAcceptStillReturnsJSON(t *testing.T) {
	for _, path := range []string{"/api/nope", "/api/nested/nope", "/debug/nope"} {
		t.Run(path, func(t *testing.T) {
			ec := newUIEcho(t)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			rec := httptest.NewRecorder()
			ec.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("got %d, want 404", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct == "text/html; charset=utf-8" {
				t.Fatalf("404 for %s was swallowed by the SPA fallback", path)
			}
		})
	}
}

// TestUnnormalisedAPIPathReturnsJSON covers request paths that only reach the
// API after normalisation. curl and browsers collapse "." and ".." before
// sending, and our own client uses hard-coded absolute paths, so these are not
// reachable from the app itself — but a reverse proxy that does not normalise
// slashes makes "//api/..." reachable, and serving HTML where the caller
// expects JSON is precisely what this guard exists to prevent.
func TestUnnormalisedAPIPathReturnsJSON(t *testing.T) {
	for _, path := range []string{
		"//api/nope",
		"/./api/nope",
		"/app/../api/nope",
		"//debug/nope",
	} {
		t.Run(path, func(t *testing.T) {
			ec := newUIEcho(t)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept", "text/html,application/xhtml+xml")
			rec := httptest.NewRecorder()
			ec.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("got %d, want 404", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct == "text/html; charset=utf-8" {
				t.Fatalf("404 for %s was swallowed by the SPA fallback", path)
			}
		})
	}
}

// TestAPILookalikePathIsStillServed is the opposite-direction guard: a plain
// prefix test would deny the index to any path merely starting with the letters
// "/api", so a client route under a name like "apiary" would wrongly 404. The
// check must be segment-aware. These go through the /app/* route because that
// is the only way into serveIndex now that the error-handler fallback is gone.
func TestAPILookalikePathIsStillServed(t *testing.T) {
	for _, path := range []string{"/app/../apiary/nope", "/app/../apis", "/app/debugger/x"} {
		t.Run(path, func(t *testing.T) {
			ec := newUIEcho(t)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept", "text/html,application/xhtml+xml")
			rec := httptest.NewRecorder()
			ec.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("got %d, want 200", rec.Code)
			}
		})
	}
}

// TestOldUIPathsReturn404 pins the removal of the error-handler SPA fallback.
// It used to answer any GET with Accept: text/html, which meant the previous
// Alpine UI's own URLs kept returning 200 — with the React index as the body.
// A stale bookmark or a cached page would then boot the app at a path the
// /app basename cannot route, and the user would see react-router's raw
// "Unexpected Application Error! 404 Not Found" rather than a clean 404.
func TestOldUIPathsReturn404(t *testing.T) {
	for _, path := range []string{"/index.html", "/js/alpine.min.js", "/some/client/route"} {
		t.Run(path, func(t *testing.T) {
			ec := newUIEcho(t)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept", "text/html,application/xhtml+xml")
			rec := httptest.NewRecorder()
			ec.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("got %d, want 404", rec.Code)
			}
		})
	}
}
