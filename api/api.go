package api

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"time"

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

	// Get data from ledger
	ec.GET("/api/files", func(c echo.Context) error {
		list := []*types.File{}
		for _, v := range ledger.CurrentData()[edgevpn.FilesLedgerKey] {
			machine := &types.File{}
			v.Unmarshal(machine)
			list = append(list, machine)
		}
		return c.JSON(http.StatusOK, list)
	})

	ec.GET("/api/summary", func(c echo.Context) error {
		files := len(ledger.CurrentData()[edgevpn.FilesLedgerKey])
		machines := len(ledger.CurrentData()[edgevpn.MachinesLedgerKey])
		users := len(ledger.CurrentData()[edgevpn.UsersLedgerKey])
		services := len(ledger.CurrentData()[edgevpn.ServicesLedgerKey])
		blockchain := ledger.Index()

		return c.JSON(http.StatusOK, struct {
			Files, Machines, Users, Services, BlockChain int
		}{files, machines, users, services, blockchain})
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

	// ec.GET("/api/blockchain", func(c echo.Context) error {
	// 	return c.JSON(http.StatusOK, ledger.BlockChain())
	// })

	ec.GET("/api/ledger", func(c echo.Context) error {
		return c.JSON(http.StatusOK, ledger.CurrentData())
	})

	ec.GET("/api/ledger/:bucket/:key", func(c echo.Context) error {
		bucket := c.Param("bucket")
		key := c.Param("key")
		return c.JSON(http.StatusOK, ledger.CurrentData()[bucket][key])
	})

	ec.GET("/api/ledger/:bucket", func(c echo.Context) error {
		bucket := c.Param("bucket")
		return c.JSON(http.StatusOK, ledger.CurrentData()[bucket])
	})

	announcing := struct{ State string }{"Announcing"}

	// Store arbitrary data
	ec.PUT("/api/ledger/:bucket/:key/:value", func(c echo.Context) error {
		bucket := c.Param("bucket")
		key := c.Param("key")
		value := c.Param("value")

		ledger.Persist(context.Background(), 5*time.Second, bucket, key, value)
		return c.JSON(http.StatusOK, announcing)
	})

	// Delete data from ledger
	ec.DELETE("/api/ledger/:bucket", func(c echo.Context) error {
		bucket := c.Param("bucket")

		ledger.AnnounceDeleteBucket(context.Background(), 5*time.Second, bucket)
		return c.JSON(http.StatusOK, announcing)
	})

	ec.DELETE("/api/ledger/:bucket/:key", func(c echo.Context) error {
		bucket := c.Param("bucket")
		key := c.Param("key")

		ledger.AnnounceDeleteBucketKey(context.Background(), 5*time.Second, bucket, key)
		return c.JSON(http.StatusOK, announcing)
	})

	return ec.Start(l)
}
