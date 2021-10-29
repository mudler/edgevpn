package api

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mudler/edgevpn/pkg/blockchain"
)

//go:embed public
var embededFiles embed.FS

func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(embededFiles, "public")
	if err != nil {
		panic(err)
	}

	return http.FS(fsys)
}

func API(l string, ledger *blockchain.Ledger) error {
	ec := echo.New()
	assetHandler := http.FileServer(getFileSystem())

	ec.GET("/api/data", func(c echo.Context) error {
		list := []blockchain.Data{}
		for _, v := range ledger.CurrentData() {
			list = append(list, v)
		}
		return c.JSON(http.StatusOK, list)
	})

	ec.GET("/*", echo.WrapHandler(http.StripPrefix("/", assetHandler)))

	ec.GET("/api/blockchain", func(c echo.Context) error {
		//		c.SetHandler()
		return c.JSON(http.StatusOK, ledger.BlockChain())
	})

	return ec.Start(l)
}
