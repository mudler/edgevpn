package edgevpn

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn/types"
)

const (
	ServicesLedgerKey = "services"
	UsersLedgerKey    = "users"
)

func (e *EdgeVPN) ExposeService(ledger *blockchain.Ledger, serviceID, virtualIP, dstaddress string) {

	e.Logger().Infof("Exposing service '%s' (%s)", serviceID, dstaddress)

	// 1) Register the ServiceID <-> PeerID Association
	// By announcing periodically our service to the blockchain
	ledger.Announce(
		context.Background(),
		e.config.LedgerAnnounceTime,
		func() {
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(ServicesLedgerKey, serviceID)
			service := &types.Service{}
			existingValue.Unmarshal(service)
			// If mismatch, update the blockchain
			if !found || service.PeerID != e.host.ID().String() {
				updatedMap := map[string]interface{}{}
				updatedMap[serviceID] = types.Service{
					PeerID:    e.host.ID().String(),
					Name:      serviceID,
					VirtualIP: virtualIP,
				}
				ledger.Add(ServicesLedgerKey, updatedMap)
			}
		},
	)

	// 2) Set a stream handler
	//    which connect to the given address/Port and Send what we receive from the Stream.
	e.config.StreamHandlers[protocol.ID(ServiceProtocol)] = func(stream network.Stream) {
		go func() {
			e.config.Logger.Infof("(service %s) Received connection from %s", serviceID, stream.Conn().RemotePeer().String())

			// Retrieve current ID for ip in the blockchain
			_, found := ledger.GetKey(UsersLedgerKey, stream.Conn().RemotePeer().String())

			// TODO: Unsticky this.
			// Maybe Services and files should have their own controllers
			if !found {
				// No user found, so we check if the connection was originated from a VPN node
				data := ledger.LastBlock().Storage[MachinesLedgerKey]
				for _, m := range data {
					machine := &types.Machine{}
					m.Unmarshal(machine)
					if machine.PeerID == stream.Conn().RemotePeer().String() {
						found = true
					}
				}

				if !found {
					// We didn't find again any match, so we close the connection
					e.config.Logger.Debugf("Reset '%s': not found in the ledger", stream.Conn().RemotePeer().String())
					stream.Reset()
					return
				}
			}

			e.config.Logger.Infof("Connecting to '%s'", dstaddress)
			c, err := net.Dial("tcp", dstaddress)
			if err != nil {
				e.config.Logger.Debugf("Reset %s: %s", stream.Conn().RemotePeer().String(), err.Error())
				stream.Reset()
				return
			}
			closer := make(chan struct{}, 2)
			go copyStream(closer, stream, c)
			go copyStream(closer, c, stream)
			<-closer

			stream.Close()
			c.Close()

			e.config.Logger.Infof("(service %s) Handled correctly '%s'", serviceID, stream.Conn().RemotePeer().String())
		}()
	}
}

func (e *EdgeVPN) ConnectToService(ledger *blockchain.Ledger, serviceID string, srcaddr string) error {

	// Open local port for listening
	l, err := net.Listen("tcp", srcaddr)
	if err != nil {
		return err
	}
	e.Logger().Info("Binding local port on", srcaddr)

	// Announce ourselves so nodes accepts our connection
	ledger.Announce(
		context.Background(),
		e.config.LedgerAnnounceTime,
		func() {
			// Retrieve current ID for ip in the blockchain
			_, found := ledger.GetKey(UsersLedgerKey, e.host.ID().String())
			// If mismatch, update the blockchain
			if !found {
				updatedMap := map[string]interface{}{}
				updatedMap[e.host.ID().String()] = &types.User{
					PeerID:    e.host.ID().String(),
					Timestamp: time.Now().String(),
				}
				ledger.Add(UsersLedgerKey, updatedMap)
			}
		},
	)
	defer l.Close()
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			e.config.Logger.Error("Error accepting: ", err.Error())
			continue
		}

		e.config.Logger.Info("New connection from", l.Addr().String())
		// Handle connections in a new goroutine, forwarding to the p2p service
		go func() {
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(ServicesLedgerKey, serviceID)
			service := &types.Service{}
			existingValue.Unmarshal(service)
			// If mismatch, update the blockchain
			if !found {
				conn.Close()
				e.config.Logger.Debugf("service '%s' not found on blockchain", serviceID)
				return
			}

			// Decode the Peer
			d, err := peer.Decode(service.PeerID)
			if err != nil {
				conn.Close()
				e.config.Logger.Debugf("could not decode peer '%s'", service.PeerID)
				return
			}

			// Open a stream
			stream, err := e.host.NewStream(context.Background(), d, ServiceProtocol)
			if err != nil {
				conn.Close()
				e.config.Logger.Debugf("could not open stream '%s'", err.Error())
				return
			}
			e.config.Logger.Debugf("(service %s) Redirecting", serviceID, l.Addr().String())

			closer := make(chan struct{}, 2)
			go copyStream(closer, stream, conn)
			go copyStream(closer, conn, stream)
			<-closer

			stream.Close()
			conn.Close()
			e.config.Logger.Infof("(service %s) Done handling %s", serviceID, l.Addr().String())
		}()
	}
}

func copyStream(closer chan struct{}, dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
	closer <- struct{}{} // connection is closed, send signal to stop proxy
}
