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
	"context"
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	apiTypes "github.com/mudler/edgevpn/api/types"

	"github.com/labstack/echo/v4"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"
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

func API(ctx context.Context, l string, defaultInterval, timeout time.Duration, e *node.Node) error {

	ledger, _ := e.Ledger()

	ec := echo.New()
	assetHandler := http.FileServer(getFileSystem())

	// Get data from ledger
	ec.GET("/api/files", func(c echo.Context) error {
		list := []*types.File{}
		for _, v := range ledger.CurrentData()[protocol.FilesLedgerKey] {
			machine := &types.File{}
			v.Unmarshal(machine)
			list = append(list, machine)
		}
		return c.JSON(http.StatusOK, list)
	})

	ec.GET("/api/summary", func(c echo.Context) error {
		files := len(ledger.CurrentData()[protocol.FilesLedgerKey])
		machines := len(ledger.CurrentData()[protocol.MachinesLedgerKey])
		users := len(ledger.CurrentData()[protocol.UsersLedgerKey])
		services := len(ledger.CurrentData()[protocol.ServicesLedgerKey])
		onChainNodes := len(e.HubRoom.Topic.ListPeers())
		p2pPeers := len(e.Host().Network().Peerstore().Peers())
		nodeID := e.Host().ID().String()

		blockchain := ledger.Index()

		return c.JSON(http.StatusOK, struct {
			Files, Machines, Users, Services, BlockChain, OnChainNodes, Peers int
			NodeID                                                            string
		}{files, machines, users, services, blockchain, onChainNodes, p2pPeers, nodeID})
	})

	ec.GET("/api/machines", func(c echo.Context) error {
		list := []*apiTypes.Machine{}
		for _, v := range ledger.CurrentData()[protocol.MachinesLedgerKey] {
			machine := &types.Machine{}
			v.Unmarshal(machine)
			m := &apiTypes.Machine{Machine: *machine}
			if e.Host().Network().Connectedness(peer.ID(machine.PeerID)) == network.Connected {
				m.Connected = true
			}
			for _, p := range e.HubRoom.Topic.ListPeers() {
				if p.String() == machine.PeerID {
					m.OnChain = true
				}
			}
			list = append(list, m)

		}

		return c.JSON(http.StatusOK, list)
	})

	ec.GET("/api/nodes", func(c echo.Context) error {
		list := []apiTypes.Peer{}
		for _, v := range e.HubRoom.Topic.ListPeers() {
			list = append(list, apiTypes.Peer{ID: v.String()})
		}

		return c.JSON(http.StatusOK, list)
	})

	ec.GET("/api/peerstore", func(c echo.Context) error {
		list := []apiTypes.Peer{}
		for _, v := range e.Host().Network().Peerstore().Peers() {
			list = append(list, apiTypes.Peer{ID: v.String()})
		}
		e.HubRoom.Topic.ListPeers()

		return c.JSON(http.StatusOK, list)
	})

	ec.GET("/api/users", func(c echo.Context) error {
		user := []*types.User{}
		for _, v := range ledger.CurrentData()[protocol.UsersLedgerKey] {
			u := &types.User{}
			v.Unmarshal(u)
			user = append(user, u)
		}
		return c.JSON(http.StatusOK, user)
	})

	ec.GET("/api/services", func(c echo.Context) error {
		list := []*types.Service{}
		for _, v := range ledger.CurrentData()[protocol.ServicesLedgerKey] {
			srvc := &types.Service{}
			v.Unmarshal(srvc)
			list = append(list, srvc)
		}
		return c.JSON(http.StatusOK, list)
	})

	ec.GET("/*", echo.WrapHandler(http.StripPrefix("/", assetHandler)))

	ec.GET("/api/blockchain", func(c echo.Context) error {
		return c.JSON(http.StatusOK, ledger.LastBlock())
	})

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

		ledger.Persist(context.Background(), defaultInterval, timeout, bucket, key, value)
		return c.JSON(http.StatusOK, announcing)
	})

	// Delete data from ledger
	ec.DELETE("/api/ledger/:bucket", func(c echo.Context) error {
		bucket := c.Param("bucket")

		ledger.AnnounceDeleteBucket(context.Background(), defaultInterval, timeout, bucket)
		return c.JSON(http.StatusOK, announcing)
	})

	ec.DELETE("/api/ledger/:bucket/:key", func(c echo.Context) error {
		bucket := c.Param("bucket")
		key := c.Param("key")

		ledger.AnnounceDeleteBucketKey(context.Background(), defaultInterval, timeout, bucket, key)
		return c.JSON(http.StatusOK, announcing)
	})

	ec.HideBanner = true

	if err := ec.Start(l); err != nil && err != http.ErrServerClosed {
		return err
	}

	go func() {
		<-ctx.Done()
		ec.Shutdown(ctx)

	}()

	return nil
}
