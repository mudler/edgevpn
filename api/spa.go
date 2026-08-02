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
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

//go:embed react-ui/dist/*
var reactUI embed.FS

// immutableAssetCacheControl is safe because Vite emits content-hashed
// filenames: a changed file always has a changed name.
const immutableAssetCacheControl = "public, max-age=31536000, immutable"

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

	serveIndex := func(c echo.Context) error {
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

	// SPA fallback. A browser navigating to a client-side route gets the
	// index; anything asking for JSON, and anything under /api, keeps the
	// real 404 so API errors are never masked by an HTML page.
	defaultHandler := ec.HTTPErrorHandler
	ec.HTTPErrorHandler = func(err error, c echo.Context) {
		he, ok := err.(*echo.HTTPError)
		if ok && he.Code == http.StatusNotFound &&
			c.Request().Method == http.MethodGet &&
			!strings.HasPrefix(c.Path(), "/api") &&
			!strings.HasPrefix(c.Request().URL.Path, "/api") &&
			!strings.HasPrefix(c.Request().URL.Path, "/debug") &&
			strings.Contains(c.Request().Header.Get("Accept"), "text/html") {
			if serveErr := serveIndex(c); serveErr == nil {
				return
			}
		}
		defaultHandler(err, c)
	}

	return nil
}
