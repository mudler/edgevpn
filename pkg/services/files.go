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

package services

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/ipfs/go-log"
	"github.com/mudler/edgevpn/pkg/protocol"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/types"
	"github.com/pkg/errors"
)

func SendFile(ctx context.Context, ledger *blockchain.Ledger, node types.Node, l log.StandardLogger, announcetime time.Duration, fileID, filepath string) error {

	l.Infof("Serving '%s' as '%s'", filepath, fileID)

	// By announcing periodically our service to the blockchain
	ledger.Announce(
		ctx,
		announcetime,
		func() {
			// Retrieve current ID for ip in the blockchain
			existingValue, found := ledger.GetKey(protocol.FilesLedgerKey, fileID)
			service := &types.Service{}
			existingValue.Unmarshal(service)
			// If mismatch, update the blockchain
			if !found || service.PeerID != node.Host().ID().String() {
				updatedMap := map[string]interface{}{}
				updatedMap[fileID] = types.File{PeerID: node.Host().ID().String(), Name: fileID}
				ledger.Add(protocol.FilesLedgerKey, updatedMap)
			}
		},
	)
	_, err := os.Stat(filepath)
	if err != nil {
		return err
	}

	// 2) Set a stream handler
	//    which connect to the given address/Port and Send what we receive from the Stream.
	node.AddStreamHandler(protocol.FileProtocol, func(stream network.Stream) {
		go func() {
			l.Infof("(file %s) Received connection from %s", fileID, stream.Conn().RemotePeer().String())

			// Retrieve current ID for ip in the blockchain
			_, found := ledger.GetKey(protocol.UsersLedgerKey, stream.Conn().RemotePeer().String())
			// If mismatch, update the blockchain
			if !found {
				l.Info("Reset", stream.Conn().RemotePeer().String(), "Not found in the ledger")
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

			l.Infof("(file %s) Done handling %s", fileID, stream.Conn().RemotePeer().String())

		}()
	})

	return nil
}

func ReceiveFile(ctx context.Context, ledger *blockchain.Ledger, node types.Node, l log.StandardLogger, announcetime time.Duration, fileID string, path string) error {
	// Announce ourselves so nodes accepts our connection
	ledger.Announce(
		ctx,
		announcetime,
		func() {
			// Retrieve current ID for ip in the blockchain
			_, found := ledger.GetKey(protocol.UsersLedgerKey, node.Host().ID().String())
			// If mismatch, update the blockchain
			if !found {
				updatedMap := map[string]interface{}{}
				updatedMap[node.Host().ID().String()] = &types.User{
					PeerID:    node.Host().ID().String(),
					Timestamp: time.Now().String(),
				}
				ledger.Add(protocol.UsersLedgerKey, updatedMap)
			}
		},
	)

	for {
		select {
		case <-ctx.Done():
			return errors.New("context canceled")
		default:
			time.Sleep(5 * time.Second)

			l.Debug("Attempting to find file in the blockchain")

			_, found := ledger.GetKey(protocol.UsersLedgerKey, node.Host().ID().String())
			if !found {
				continue
			}
			existingValue, found := ledger.GetKey(protocol.FilesLedgerKey, fileID)
			fi := &types.File{}
			existingValue.Unmarshal(fi)
			// If mismatch, update the blockchain
			if !found {
				l.Debug("file not found on blockchain, retrying in 5 seconds")
				continue
			} else {
				// Retrieve current ID for ip in the blockchain
				existingValue, found := ledger.GetKey(protocol.FilesLedgerKey, fileID)
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
				stream, err := node.Host().NewStream(context.Background(), d, protocol.FileProtocol.ID())
				if err != nil {
					l.Debugf("failed to dial %s, retrying in 5 seconds", d)
					continue
				}

				l.Infof("Saving file %s to %s", fileID, path)

				f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
				if err != nil {
					return err
				}

				io.Copy(f, stream)

				f.Close()

				l.Infof("Received file %s to %s", fileID, path)
				return nil
			}
		}
	}
}
