// Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package api

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

//go:embed react-ui/dist/*
var reactUI embed.FS

// immutableAssetCacheControl is safe because Vite emits content-hashed
// filenames: a changed file always has a changed name.
const immutableAssetCacheControl = "public, max-age=31536000, immutable"

// isUnder reports whether p addresses base or something beneath it.
//
// It normalises first: a request path is attacker-controlled and reaches us
// verbatim, so "//api/x", "/./api/x" and "/app/../api/x" all name the API
// without literally starting with "/api". It then compares whole segments, so
// "/apiary" is not treated as living under "/api".
func isUnder(p, base string) bool {
	if p == "" {
		return false
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = path.Clean(p)
	return p == base || strings.HasPrefix(p, base+"/")
}

// registerUI wires the embedded React application into ec.
//
// It returns an error only when the embedded filesystem is unusable. The
// caller is expected to log and continue rather than abort — a binary
// built without the UI must still serve the JSON API.
func registerUI(ec *echo.Echo) error {
	uiFS, err := fs.Sub(reactUI, "react-ui/dist")
	if err != nil {
		return err
	}

	// reserved reports whether this request names the JSON API or the pprof
	// endpoints, and so must never be answered with the index page.
	reserved := func(c echo.Context) bool {
		for _, base := range []string{"/api", "/debug"} {
			if isUnder(c.Path(), base) || isUnder(c.Request().URL.Path, base) {
				return true
			}
		}
		return false
	}

	serveIndex := func(c echo.Context) error {
		// "/app/../api/x" matches the /app/* route below yet names the API
		// once normalised, so it arrives here and must not be answered with
		// an HTML page where the caller is expecting JSON.
		if reserved(c) {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		index, err := fs.ReadFile(uiFS, "index.html")
		if err != nil {
			return c.String(http.StatusNotFound, "React UI not built")
		}
		// The index must never be cached: it names the hashed asset
		// bundles, so a stale copy points at files that no longer exist.
		c.Response().Header().Set("Cache-Control", "no-cache")
		return c.HTMLBlob(http.StatusOK, index)
	}

	ec.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/app")
	})
	ec.GET("/app", serveIndex)
	ec.GET("/app/*", serveIndex)

	ec.GET("/assets/*", func(c echo.Context) error {
		name := "assets/" + c.Param("*")
		body, err := fs.ReadFile(uiFS, name)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		c.Response().Header().Set("Cache-Control", immutableAssetCacheControl)
		ctype := mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		return c.Blob(http.StatusOK, ctype, body)
	})

	// Anything else at the root that is not under /api or /debug — the
	// favicon, for instance — comes straight from the bundle.
	ec.GET("/favicon.svg", func(c echo.Context) error {
		body, err := fs.ReadFile(uiFS, "favicon.svg")
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		return c.Blob(http.StatusOK, "image/svg+xml", body)
	})

	// No catch-all SPA fallback on the error handler. Client-side routing
	// needs none: /app and /app/* are real routes, so every deep link the
	// router can match is already served above. A fallback would instead
	// answer the *old* UI's URLs — /index.html, /js/alpine.min.js — with the
	// React index, so a stale bookmark or a cached page would load the app at
	// a path the /app basename cannot route and surface react-router's raw
	// "Unexpected Application Error! 404 Not Found" instead of a clean 404.
	return nil
}
