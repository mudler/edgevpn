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
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	p2pprotocol "github.com/libp2p/go-libp2p/core/protocol"

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
	MetricsURL    = "/api/metrics"
	PeerstoreURL  = "/api/peerstore"
	PeerGateURL   = "/api/peergate"

	// UnixSocketScheme is the URI prefix that selects a unix domain
	// socket listener for the API instead of a TCP address.
	UnixSocketScheme = "unix://"

	// defaultUnixSocketMode is the file mode applied to API unix
	// sockets when none is supplied via the APILISTENUNIXMODE env var.
	// 0660 lets the owner and group communicate with the API and
	// blocks every other process on the host, which is the right
	// default for a hardened local control plane.
	defaultUnixSocketMode os.FileMode = 0o660
)

// unixSocketMode resolves the file mode that should be applied to
// the API socket. APILISTENUNIXMODE is interpreted as an octal value
// (e.g. "0600"); anything unparseable falls back to defaultUnixSocketMode
// so a typo cannot accidentally widen permissions.
func unixSocketMode() os.FileMode {
	v := strings.TrimSpace(os.Getenv("APILISTENUNIXMODE"))
	if v == "" {
		return defaultUnixSocketMode
	}
	// Accept either "0600" or "600" forms.
	parsed, err := parseOctal(v)
	if err != nil {
		return defaultUnixSocketMode
	}
	return os.FileMode(parsed) & os.ModePerm
}

func parseOctal(s string) (uint32, error) {
	if s == "" {
		return 0, errors.New("empty mode")
	}
	var n uint32
	// Strip an optional leading "0" / "0o" so callers can write the
	// usual unix shorthand without us forcing a specific notation.
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0o"), "0O")
	for _, r := range s {
		if r < '0' || r > '7' {
			return 0, fmt.Errorf("invalid octal digit %q", r)
		}
		n = n<<3 | uint32(r-'0')
	}
	return n, nil
}

// systemdSocketListener returns a unix listener inherited from systemd
// (LISTEN_FDS/LISTEN_PID), if any. nil, nil means "no socket activation
// in effect" — the caller should fall through to its normal listen path.
//
// Inheriting the listener is the right path for the operator workflow
// where a .socket unit declares the path, user, group and mode and
// systemd hands the already-bound FD to the service. In that case we
// must NOT touch the underlying socket file (its ownership and perms
// are systemd's responsibility) and we must NOT chmod after the fact
// (the kernel mode already came from the .socket unit).
//
// Reference: sd_listen_fds(3). The convention is FD 3 for the first
// inherited socket and LISTEN_FDS counts how many are present. We only
// support a single API listener so we accept LISTEN_FDS=1 strictly.
func systemdSocketListener() (net.Listener, error) {
	pidEnv := os.Getenv("LISTEN_PID")
	fdsEnv := os.Getenv("LISTEN_FDS")
	if pidEnv == "" || fdsEnv == "" {
		return nil, nil
	}
	pid, err := strconv.Atoi(pidEnv)
	if err != nil {
		return nil, fmt.Errorf("invalid LISTEN_PID %q: %w", pidEnv, err)
	}
	if pid != os.Getpid() {
		// systemd-managed FDs are addressed to a specific PID;
		// if it isn't us, treat the env as not present.
		return nil, nil
	}
	fds, err := strconv.Atoi(fdsEnv)
	if err != nil {
		return nil, fmt.Errorf("invalid LISTEN_FDS %q: %w", fdsEnv, err)
	}
	if fds != 1 {
		return nil, fmt.Errorf("LISTEN_FDS=%d, expected exactly 1 (only the API socket can be passed)", fds)
	}
	// First inherited FD is always 3 (after stdin/stdout/stderr).
	const firstFD = 3
	// Don't leak the FD as O_CLOEXEC-off to children spawned later.
	syscall.CloseOnExec(firstFD)
	f := os.NewFile(uintptr(firstFD), "systemd-edgevpn-api-socket")
	if f == nil {
		return nil, fmt.Errorf("could not wrap inherited fd %d", firstFD)
	}
	l, err := net.FileListener(f)
	// FileListener dups the fd; close ours so we don't keep it open twice.
	f.Close()
	if err != nil {
		return nil, fmt.Errorf("net.FileListener(fd=%d): %w", firstFD, err)
	}
	return l, nil
}

// unixSocketInUse probes whether something is already serving on the
// socket at path. A successful connect — even an immediate hangup —
// means a listener is present and we should refuse to clobber it. We
// treat ECONNREFUSED as "stale socket file from a crashed previous
// run" and any other dial error conservatively as "in use" so we
// never unlink a working socket on a flaky filesystem.
func unixSocketInUse(path string) bool {
	c, err := net.DialTimeout("unix", path, 250*time.Millisecond)
	if err == nil {
		c.Close()
		return true
	}
	// errors.Is(err, syscall.ECONNREFUSED) is the only signal we'll
	// accept as "definitively not in use".
	return !errors.Is(err, syscall.ECONNREFUSED)
}

// listenUnix creates the API unix socket listener. It only removes a
// stale socket file (one whose owning process is gone) and then
// tightens permissions before any client can connect — chmod must
// happen between Listen and the first Accept to close the window
// where the kernel-default mode is observable.
//
// If the socket is currently being served, listenUnix refuses to
// touch it — this protects against a second edgevpn racing into a
// path already owned by a running instance, and against an operator
// who pre-created the socket file (e.g. via a systemd .socket unit
// without activation, or `install -m 0660 -o edgevpn`) and intends
// edgevpn to use what's there. For full systemd socket-activation
// support, see systemdSocketListener which is invoked by API() before
// this function ever runs.
func listenUnix(path string) (net.Listener, error) {
	if fi, err := os.Lstat(path); err == nil {
		if fi.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("refusing to remove non-socket file at %s", path)
		}
		if unixSocketInUse(path) {
			return nil, fmt.Errorf("socket %s is already in use by another process", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale socket %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat socket %s: %w", path, err)
	}

	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, unixSocketMode()); err != nil {
		l.Close()
		return nil, fmt.Errorf("chmod socket %s: %w", path, err)
	}
	return l, nil
}

func API(ctx context.Context, l string, defaultInterval, timeout time.Duration, e *node.Node, bwc metrics.Reporter, debugMode bool) error {

	ledger, _ := e.Ledger()

	ec := echo.New()

	var (
		unixSocketPath  string
		ownsSocketFile  bool // true iff WE created the file; false for systemd-passed FDs
	)
	if strings.HasPrefix(l, UnixSocketScheme) {
		unixSocketPath = strings.TrimPrefix(l, UnixSocketScheme)
		// Honour systemd socket activation first: if the operator
		// declared a .socket unit, systemd has already created and
		// bound the socket with the user/group/mode they want, and
		// passed us its FD. Inherit that listener verbatim — do NOT
		// touch the underlying file. systemdSocketListener returns
		// (nil, nil) when no activation is in effect, in which case
		// we fall through to listenUnix and manage the socket file
		// ourselves.
		unixListener, err := systemdSocketListener()
		if err != nil {
			return fmt.Errorf("systemd socket activation: %w", err)
		}
		if unixListener == nil {
			unixListener, err = listenUnix(unixSocketPath)
			if err != nil {
				return err
			}
			ownsSocketFile = true
		}
		ec.Listener = unixListener
	}

	assetHandler := http.FileServer(getFileSystem())
	if debugMode {
		ec.GET("/debug/pprof/*", echo.WrapHandler(http.DefaultServeMux))
	}

	if bwc != nil {
		ec.GET(MetricsURL, func(c echo.Context) error {
			return c.JSON(http.StatusOK, bwc.GetBandwidthTotals())
		})
		ec.GET(filepath.Join(MetricsURL, "protocol"), func(c echo.Context) error {
			return c.JSON(http.StatusOK, bwc.GetBandwidthByProtocol())
		})
		ec.GET(filepath.Join(MetricsURL, "peer"), func(c echo.Context) error {
			return c.JSON(http.StatusOK, bwc.GetBandwidthByPeer())
		})
		ec.GET(filepath.Join(MetricsURL, "peer", ":peer"), func(c echo.Context) error {
			return c.JSON(http.StatusOK, bwc.GetBandwidthForPeer(peer.ID(c.Param("peer"))))
		})
		ec.GET(filepath.Join(MetricsURL, "protocol", ":protocol"), func(c echo.Context) error {
			return c.JSON(http.StatusOK, bwc.GetBandwidthForProtocol(p2pprotocol.ID(c.Param("protocol"))))
		})
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

	if e.PeerGater() != nil {
		ec.PUT(fmt.Sprintf("%s/:state", PeerGateURL), func(c echo.Context) error {
			state := c.Param("state")

			switch state {
			case "enable":
				e.PeerGater().Enable()
			case "disable":
				e.PeerGater().Disable()
			}
			return c.JSON(http.StatusOK, e.PeerGater().Enabled())
		})

		ec.GET(PeerGateURL, func(c echo.Context) error {
			return c.JSON(http.StatusOK, e.PeerGater().Enabled())
		})
	}

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

	// Tie the server's lifetime to ctx so callers can stop us from
	// the outside. Registering the watcher BEFORE ec.Start avoids
	// the dead-goroutine pattern where Shutdown was scheduled after
	// the blocking call.
	go func() {
		<-ctx.Done()
		ct, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = ec.Shutdown(ct)
	}()

	startErr := ec.Start(l)

	// Remove the socket on exit only when we own the file. With
	// systemd socket activation the .socket unit owns the path
	// (permissions, ownership, lifecycle) and systemd is responsible
	// for cleanup; unlinking it from here would break the next
	// activation cycle.
	if ownsSocketFile && unixSocketPath != "" {
		_ = os.Remove(unixSocketPath)
	}

	if startErr != nil && !errors.Is(startErr, http.ErrServerClosed) {
		return startErr
	}
	return nil
}
