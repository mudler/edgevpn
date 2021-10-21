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
