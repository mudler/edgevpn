package edgevpn

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn/types"
	"github.com/pkg/errors"
)

const (
	FilesLedgerKey = "files"
)

func (e *EdgeVPN) SendFile(ledger *blockchain.Ledger, fileID, filepath string) error {

	e.Logger().Infof("Serving '%s' as '%s", filepath, fileID)

	// By announcing periodically our service to the blockchain
	ledger.Announce(
		context.Background(),
		e.config.LedgerAnnounceTime,
		func() {
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(FilesLedgerKey, fileID)
			service := &types.Service{}
			existingValue.Unmarshal(service)
			// If mismatch, update the blockchain
			if !found || service.PeerID != e.host.ID().String() {
				updatedMap := map[string]interface{}{}
				updatedMap[fileID] = types.File{PeerID: e.host.ID().String(), Name: fileID}
				ledger.Add(FilesLedgerKey, updatedMap)
			}
		},
	)
	_, err := os.Stat(filepath)
	if err != nil {
		return err
	}
	// 2) Set a stream handler
	//    which connect to the given address/Port and Send what we receive from the Stream.
	e.config.StreamHandlers[protocol.ID(FileProtocol)] = func(stream network.Stream) {
		go func() {
			e.config.Logger.Infof("(file %s) Received connection from %s", fileID, stream.Conn().RemotePeer().String())

			// Retrieve current ID for ip in the blockchain
			_, found := ledger.GetKey(UsersLedgerKey, stream.Conn().RemotePeer().String())
			// If mismatch, update the blockchain
			if !found {
				e.config.Logger.Info("Reset", stream.Conn().RemotePeer().String(), "Not found in the ledger")
				stream.Reset()
				return
			}
			f, err := os.Open(filepath)
			if err != nil {
				return
			}
			io.Copy(stream, f)
			f.Close()

			stream.Close()

			e.config.Logger.Infof("(file %s) Done handling %s", fileID, stream.Conn().RemotePeer().String())

		}()
	}

	return nil
}

func (e *EdgeVPN) ReceiveFile(ledger *blockchain.Ledger, fileID string, path string) error {

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
	for {
		time.Sleep(5 * time.Second)

		e.config.Logger.Debug("Attempting to find file in the blockchain")

		_, found := ledger.GetKey(UsersLedgerKey, e.host.ID().String())
		if !found {
			continue
		}
		existingValue, found := ledger.GetKey(FilesLedgerKey, fileID)
		fi := &types.File{}
		existingValue.Unmarshal(fi)
		// If mismatch, update the blockchain
		if !found {
			e.config.Logger.Debug("file not found on blockchain, retrying in 5 seconds")
			continue
		} else {
			break
		}
	}
	// Listen for an incoming connection.

	// Retrieve current ID for ip in the blockchain
	existingValue, found := ledger.GetKey(FilesLedgerKey, fileID)
	fi := &types.File{}
	existingValue.Unmarshal(fi)

	// If mismatch, update the blockchain
	if !found {
		return errors.New("file not found")
	}

	// Decode the Peer
	d, err := peer.Decode(fi.PeerID)
	if err != nil {
		return err
	}

	// Open a stream
	stream, err := e.host.NewStream(context.Background(), d, FileProtocol)
	if err != nil {
		return err
	}
	e.config.Logger.Infof("Saving file %s to %s", fileID, path)

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	io.Copy(f, stream)

	f.Close()

	e.config.Logger.Infof("Received file %s to %s", fileID, path)
	return nil
}
