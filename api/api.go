package api

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/mudler/edgevpn/pkg/edgevpn/types"
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

	ec.GET("/api/files", func(c echo.Context) error {
		list := []*types.File{}
		for _, v := range ledger.CurrentData()[edgevpn.FilesLedgerKey] {
			machine := &types.File{}
			v.Unmarshal(machine)
			list = append(list, machine)
		}
		return c.JSON(http.StatusOK, list)
	})

	ec.GET("/api/machines", func(c echo.Context) error {
		list := []*types.Machine{}
		for _, v := range ledger.CurrentData()[edgevpn.MachinesLedgerKey] {
			machine := &types.Machine{}
			v.Unmarshal(machine)
			list = append(list, machine)
		}
		return c.JSON(http.StatusOK, list)
	})

	ec.GET("/api/users", func(c echo.Context) error {
		user := []*types.User{}
		for _, v := range ledger.CurrentData()[edgevpn.UsersLedgerKey] {
			u := &types.User{}
			v.Unmarshal(u)
			user = append(user, u)
		}
		return c.JSON(http.StatusOK, user)
	})

	ec.GET("/api/services", func(c echo.Context) error {
		list := []*types.Service{}
		for _, v := range ledger.CurrentData()[edgevpn.ServicesLedgerKey] {
			srvc := &types.Service{}
			v.Unmarshal(srvc)
			list = append(list, srvc)
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
