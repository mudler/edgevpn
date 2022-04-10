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
	"fmt"
	"io/fs"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/miekg/dns"
	apiTypes "github.com/mudler/edgevpn/api/types"

	"github.com/labstack/echo/v4"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/services"
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

const (
	MachineURL    = "/api/machines"
	UsersURL      = "/api/users"
	ServiceURL    = "/api/services"
	BlockchainURL = "/api/blockchain"
	LedgerURL     = "/api/ledger"
	SummaryURL    = "/api/summary"
	FileURL       = "/api/files"
	NodesURL      = "/api/nodes"
	DNSURL        = "/api/dns"
	PeerstoreURL  = "/api/peerstore"
)

func API(ctx context.Context, l string, defaultInterval, timeout time.Duration, e *node.Node, debugMode bool) error {

	ledger, _ := e.Ledger()

	ec := echo.New()

	if strings.HasPrefix(l, "unix://") {
		unixListener, err := net.Listen("unix", strings.ReplaceAll(l, "unix://", ""))
		if err != nil {
			return err
		}
		ec.Listener = unixListener
	}

	assetHandler := http.FileServer(getFileSystem())
	if debugMode {
		ec.GET("/debug/pprof/*", echo.WrapHandler(http.DefaultServeMux))
	}

	// Get data from ledger
	ec.GET(FileURL, func(c echo.Context) error {
		list := []*types.File{}
		for _, v := range ledger.CurrentData()[protocol.FilesLedgerKey] {
			machine := &types.File{}
			v.Unmarshal(machine)
			list = append(list, machine)
		}
		return c.JSON(http.StatusOK, list)
	})

	ec.GET(SummaryURL, func(c echo.Context) error {
		files := len(ledger.CurrentData()[protocol.FilesLedgerKey])
		machines := len(ledger.CurrentData()[protocol.MachinesLedgerKey])
		users := len(ledger.CurrentData()[protocol.UsersLedgerKey])
		services := len(ledger.CurrentData()[protocol.ServicesLedgerKey])
		peers, err := e.MessageHub.ListPeers()
		if err != nil {
			return err
		}
		onChainNodes := len(peers)
		p2pPeers := len(e.Host().Network().Peerstore().Peers())
		nodeID := e.Host().ID().String()

		blockchain := ledger.Index()

		return c.JSON(http.StatusOK, types.Summary{
			Files:        files,
			Machines:     machines,
			Users:        users,
			Services:     services,
			BlockChain:   blockchain,
			OnChainNodes: onChainNodes,
			Peers:        p2pPeers,
			NodeID:       nodeID,
		})
	})

	ec.GET(MachineURL, func(c echo.Context) error {
		list := []*apiTypes.Machine{}

		online := services.AvailableNodes(ledger, 20*time.Minute)

		for _, v := range ledger.CurrentData()[protocol.MachinesLedgerKey] {
			machine := &types.Machine{}
			v.Unmarshal(machine)
			m := &apiTypes.Machine{Machine: *machine}
			if e.Host().Network().Connectedness(peer.ID(machine.PeerID)) == network.Connected {
				m.Connected = true
			}
			peers, err := e.MessageHub.ListPeers()
			if err != nil {
				return err
			}
			for _, p := range peers {
				if p.String() == machine.PeerID {
					m.OnChain = true
				}
			}
			for _, a := range online {
				if a == machine.PeerID {
					m.Online = true
				}
			}
			list = append(list, m)

		}

		return c.JSON(http.StatusOK, list)
	})

	ec.GET(NodesURL, func(c echo.Context) error {
		list := []apiTypes.Peer{}
		peers, err := e.MessageHub.ListPeers()
		if err != nil {
			return err
		}

		// Sum up state also from services
		online := services.AvailableNodes(ledger, 10*time.Minute)
		p := map[string]interface{}{}

		for _, v := range online {
			p[v] = nil
		}

		for _, v := range peers {
			_, exists := p[v.String()]
			if !exists {
				p[v.String()] = nil
			}
		}

		for id, _ := range p {
			list = append(list, apiTypes.Peer{ID: id, Online: true})
		}

		return c.JSON(http.StatusOK, list)
	})

	ec.GET(PeerstoreURL, func(c echo.Context) error {
		list := []apiTypes.Peer{}
		for _, v := range e.Host().Network().Peerstore().Peers() {
			list = append(list, apiTypes.Peer{ID: v.String()})
		}
		return c.JSON(http.StatusOK, list)
	})

	ec.GET(UsersURL, func(c echo.Context) error {
		user := []*types.User{}
		for _, v := range ledger.CurrentData()[protocol.UsersLedgerKey] {
			u := &types.User{}
			v.Unmarshal(u)
			user = append(user, u)
		}
		return c.JSON(http.StatusOK, user)
	})

	ec.GET(ServiceURL, func(c echo.Context) error {
		list := []*types.Service{}
		for _, v := range ledger.CurrentData()[protocol.ServicesLedgerKey] {
			srvc := &types.Service{}
			v.Unmarshal(srvc)
			list = append(list, srvc)
		}
		return c.JSON(http.StatusOK, list)
	})

	ec.GET("/*", echo.WrapHandler(http.StripPrefix("/", assetHandler)))

	ec.GET(BlockchainURL, func(c echo.Context) error {
		return c.JSON(http.StatusOK, ledger.LastBlock())
	})

	ec.GET(LedgerURL, func(c echo.Context) error {
		return c.JSON(http.StatusOK, ledger.CurrentData())
	})

	ec.GET(fmt.Sprintf("%s/:bucket/:key", LedgerURL), func(c echo.Context) error {
		bucket := c.Param("bucket")
		key := c.Param("key")
		return c.JSON(http.StatusOK, ledger.CurrentData()[bucket][key])
	})

	ec.GET(fmt.Sprintf("%s/:bucket", LedgerURL), func(c echo.Context) error {
		bucket := c.Param("bucket")
		return c.JSON(http.StatusOK, ledger.CurrentData()[bucket])
	})

	announcing := struct{ State string }{"Announcing"}

	// Store arbitrary data
	ec.PUT(fmt.Sprintf("%s/:bucket/:key/:value", LedgerURL), func(c echo.Context) error {
		bucket := c.Param("bucket")
		key := c.Param("key")
		value := c.Param("value")

		ledger.Persist(context.Background(), defaultInterval, timeout, bucket, key, value)
		return c.JSON(http.StatusOK, announcing)
	})

	ec.GET(DNSURL, func(c echo.Context) error {
		res := []apiTypes.DNS{}
		for r, e := range ledger.CurrentData()[protocol.DNSKey] {
			var t types.DNS
			e.Unmarshal(&t)
			d := map[string]string{}

			for k, v := range t {
				d[dns.TypeToString[uint16(k)]] = v
			}

			res = append(res,
				apiTypes.DNS{
					Regex:   r,
					Records: d,
				})
		}
		return c.JSON(http.StatusOK, res)
	})

	// Announce dns
	ec.POST(DNSURL, func(c echo.Context) error {
		d := new(apiTypes.DNS)
		if err := c.Bind(d); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		entry := make(types.DNS)
		for r, e := range d.Records {
			entry[dns.Type(dns.StringToType[r])] = e
		}
		services.PersistDNSRecord(context.Background(), ledger, defaultInterval, timeout, d.Regex, entry)
		return c.JSON(http.StatusOK, announcing)
	})

	// Delete data from ledger
	ec.DELETE(fmt.Sprintf("%s/:bucket", LedgerURL), func(c echo.Context) error {
		bucket := c.Param("bucket")

		ledger.AnnounceDeleteBucket(context.Background(), defaultInterval, timeout, bucket)
		return c.JSON(http.StatusOK, announcing)
	})

	ec.DELETE(fmt.Sprintf("%s/:bucket/:key", LedgerURL), func(c echo.Context) error {
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
		ct, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		ec.Shutdown(ct)
		cancel()
	}()

	return nil
}
