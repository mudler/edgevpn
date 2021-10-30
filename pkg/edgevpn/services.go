package edgevpn

import (
	"context"
	"io"
	"net"

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

func (e *EdgeVPN) ExposeService(ledger *blockchain.Ledger, serviceID, dstaddress string) {
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
				updatedMap[serviceID] = types.Service{PeerID: e.host.ID().String(), Name: serviceID}
				ledger.Add(ServicesLedgerKey, updatedMap)
			}
		},
	)

	// 2) Set a stream handler
	//    which connect to the given address/Port and Send what we receive from the Stream.
	e.config.StreamHandlers[protocol.ID(ServiceProtocol)] = func(stream network.Stream) {
		go func() {
			e.config.Logger.Sugar().Info("Received connection from", stream.Conn().RemotePeer().String())

			// Retrieve current ID for ip in the blockchain
			_, found := ledger.GetKey(UsersLedgerKey, stream.Conn().RemotePeer().String())
			// If mismatch, update the blockchain
			if !found {
				e.config.Logger.Sugar().Info("Reset", stream.Conn().RemotePeer().String(), "Not found in the ledger")
				stream.Reset()
				return
			}

			// we need a list of known peers
			e.config.Logger.Sugar().Info("Dialing", dstaddress)

			c, err := net.Dial("tcp", dstaddress)
			if err != nil {
				e.config.Logger.Sugar().Info("Reset", stream.Conn().RemotePeer().String(), err.Error())
				stream.Reset()
				return
			}
			closer := make(chan struct{}, 2)
			go copyStream(closer, stream, c)
			go copyStream(closer, c, stream)
			<-closer

			stream.Close()
			c.Close()

			e.config.Logger.Sugar().Info("Done", stream.Conn().RemotePeer().String())

		}()
	}
}

func (e *EdgeVPN) ConnectToService(ledger *blockchain.Ledger, serviceID string, srcaddr string) error {
	// Open local port for listening
	l, err := net.Listen("tcp", srcaddr)
	if err != nil {
		return err
	}
	e.config.Logger.Sugar().Info("Listening on ", srcaddr)

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
				updatedMap[e.host.ID().String()] = ""
				ledger.Add(UsersLedgerKey, updatedMap)
			}
		},
	)
	defer l.Close()
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			e.config.Logger.Sugar().Error("Error accepting: ", err.Error())
			continue
		}
		e.config.Logger.Sugar().Info("New connection from", l.Addr().String())
		// Handle connections in a new goroutine, forwarding to the p2p service
		go func() {
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(ServicesLedgerKey, serviceID)
			service := &types.Service{}
			existingValue.Unmarshal(service)
			// If mismatch, update the blockchain
			if !found {
				e.config.Logger.Sugar().Info("service not found on blockchain")
				return
			}
			// Decode the Peer
			d, err := peer.Decode(service.PeerID)
			if err != nil {
				e.config.Logger.Sugar().Infof("could not decode peer '%s'", service.PeerID)
				return
			}

			// Open a stream
			stream, err := e.host.NewStream(context.Background(), d, ServiceProtocol)
			if err != nil {
				e.config.Logger.Sugar().Infof("could not open stream '%s'", err.Error())
				return
			}
			e.config.Logger.Sugar().Info("Redirecting", l.Addr().String(), "to", serviceID)

			closer := make(chan struct{}, 2)
			go copyStream(closer, stream, conn)
			go copyStream(closer, conn, stream)
			<-closer

			stream.Close()
			conn.Close()
			e.config.Logger.Sugar().Info("Done handling", l.Addr().String(), "to", serviceID)
		}()
	}
}

func copyStream(closer chan struct{}, dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
	closer <- struct{}{} // connection is closed, send signal to stop proxy
}
