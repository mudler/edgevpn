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

package edgevpn

import (
	"fmt"

	hub "github.com/mudler/edgevpn/pkg/hub"
)

type MessageWriter struct {
	input chan<- *hub.Message
	c     Config
	mess  *hub.Message
}

func Message(template string, opts ...interface{}) []byte {
	return []byte(fmt.Sprintf(template, opts...))
}

func (mw *MessageWriter) Write(p []byte) (n int, err error) {
	return mw.Send(mw.mess.WithMessage(string(p)))
}

func (mw *MessageWriter) WriteString(p string) (n int, err error) {
	return mw.Send(mw.mess.WithMessage(p))
}

func (mw *MessageWriter) Send(copy *hub.Message) (n int, err error) {
	mw.input <- copy
	return len(copy.Message), nil
}
