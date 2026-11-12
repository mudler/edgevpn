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

package hub

import (
	"context"
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/peer"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// Room represents a subscription to a single PubSub topic. Messages
// can be published to the topic with Room.Publish, and received
// messages are pushed to the Messages channel.
type room struct {
	ctx   context.Context
	ps    *pubsub.PubSub
	Topic *pubsub.Topic
	sub   *pubsub.Subscription

	roomName string
	self     peer.ID
}

// connect tries to subscribe to the PubSub topic for the room name, returning
// a Room on success.
func connect(ctx context.Context, ps *pubsub.PubSub, selfID peer.ID, roomName string, messageChan chan *Message) (*room, error) {
	// join the pubsub topic
	topic, err := ps.Join(roomName)
	if err != nil {
		return nil, err
	}

	// and subscribe to it
	sub, err := topic.Subscribe()
	if err != nil {
		topic.Close()
		return nil, err
	}

	cr := &room{
		ctx:      ctx,
		ps:       ps,
		Topic:    topic,
		sub:      sub,
		self:     selfID,
		roomName: roomName,
	}

	// start reading messages from the subscription in a loop
	go cr.readLoop(messageChan)
	return cr, nil
}

// leave unsubscribes from the room's topic and releases it, so the pubsub
// instance tells its peers it left and keeps no state for the topic.
func (cr *room) leave() {
	cr.sub.Cancel()
	// Close fails while the topic has subscriptions, event handlers or relays,
	// none of which the room creates besides the subscription cancelled above,
	// and after the pubsub instance has shut down, when nothing is left to
	// release.
	_ = cr.Topic.Close()
}

// publishMessage sends a message to the pubsub topic.
func (cr *room) publishMessage(m *Message) error {
	msgBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return cr.Topic.Publish(cr.ctx, msgBytes)
}

// readLoop pulls messages from the pubsub topic and pushes them onto the Messages channel.
func (cr *room) readLoop(messageChan chan *Message) {
	for {
		msg, err := cr.sub.Next(cr.ctx)
		if err != nil {
			return
		}
		// only forward messages published by others
		if msg.GetFrom() == cr.self {
			continue
		}
		cm := new(Message)
		err = json.Unmarshal(msg.Data, cm)
		if err != nil {
			continue
		}

		// The sender is the peer that published the message, which pubsub
		// verifies against the message signature. ReceivedFrom is only the
		// last peer that passed it on.
		cm.SenderID = msg.GetFrom().String()

		// send valid messages onto the Messages channel
		messageChan <- cm
	}
}
