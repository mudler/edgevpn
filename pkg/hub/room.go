// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

	"github.com/libp2p/go-libp2p-core/peer"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// RoomBufSize is the number of incoming messages to buffer for each topic.
const RoomBufSize = 128

// Room represents a subscription to a single PubSub topic. Messages
// can be published to the topic with Room.Publish, and received
// messages are pushed to the Messages channel.
type Room struct {
	// Messages is a channel of messages received from other peers in the chat room
	Messages chan *Message

	ctx   context.Context
	ps    *pubsub.PubSub
	topic *pubsub.Topic
	sub   *pubsub.Subscription

	roomName string
	self     peer.ID
	nick     string
}

// JoinRoom tries to subscribe to the PubSub topic for the room name, returning
// a Room on success.
func JoinRoom(ctx context.Context, ps *pubsub.PubSub, selfID peer.ID, roomName string) (*Room, error) {
	// join the pubsub topic
	topic, err := ps.Join(topicName(roomName))
	if err != nil {
		return nil, err
	}

	// and subscribe to it
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	cr := &Room{
		ctx:      ctx,
		ps:       ps,
		topic:    topic,
		sub:      sub,
		self:     selfID,
		roomName: roomName,
		Messages: make(chan *Message, RoomBufSize),
	}

	// start reading messages from the subscription in a loop
	go cr.readLoop()
	return cr, nil
}

// Publish sends a message to the pubsub topic.
func (cr *Room) Publish(message string, o ...func(*Message)) error {
	m := &Message{
		Message: message,
	}

	for _, f := range o {
		f(m)
	}

	return cr.PublishMessage(m)
}

// Publish sends a message to the pubsub topic.
func (cr *Room) PublishMessage(m *Message) error {
	m.SenderID = cr.self.Pretty()

	msgBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return cr.topic.Publish(cr.ctx, msgBytes)
}

// readLoop pulls messages from the pubsub topic and pushes them onto the Messages channel.
func (cr *Room) readLoop() {
	for {
		msg, err := cr.sub.Next(cr.ctx)
		if err != nil {
			close(cr.Messages)
			return
		}
		// only forward messages delivered by others
		if msg.ReceivedFrom == cr.self {
			continue
		}
		cm := new(Message)
		err = json.Unmarshal(msg.Data, cm)
		if err != nil {
			continue
		}
		// send valid messages onto the Messages channel
		cr.Messages <- cm
	}
}

func topicName(roomName string) string {
	return "chat-room:" + roomName
}
