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

package node

import (
	hub "github.com/mudler/edgevpn/pkg/hub"
)

// messageWriter is a struct returned by the node that satisfies the io.Writer interface
// on the underlying hub.
// Everything Write into the message writer is enqueued to a message channel
// which is sealed and processed by the node
type messageWriter struct {
	input chan<- *hub.Message
	c     Config
	mess  *hub.Message
}

// Write writes a slice of bytes to the message channel
func (mw *messageWriter) Write(p []byte) (n int, err error) {
	return mw.Send(mw.mess.WithMessage(string(p)))
}

// WriteString writes a string to the message channel
func (mw *messageWriter) WriteString(p string) (n int, err error) {
	return mw.Send(mw.mess.WithMessage(p))
}

// Send sends a message to the channel
func (mw *messageWriter) Send(copy *hub.Message) (n int, err error) {
	mw.input <- copy
	return len(copy.Message), nil
}
