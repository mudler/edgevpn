/*
Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package node

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/crypto"
	"github.com/mudler/edgevpn/pkg/hub"
)

// warnRecorder keeps the warnings a node logs and drops everything else.
type warnRecorder struct {
	mu    sync.Mutex
	warns []string
}

func (r *warnRecorder) Warn(args ...interface{}) { r.Warnf("%s", fmt.Sprint(args...)) }
func (r *warnRecorder) Warnf(f string, args ...interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.warns = append(r.warns, fmt.Sprintf(f, args...))
}
func (r *warnRecorder) Warning(args ...interface{})            { r.Warn(args...) }
func (r *warnRecorder) Warningf(f string, args ...interface{}) { r.Warnf(f, args...) }
func (r *warnRecorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.warns...)
}
func (r *warnRecorder) Debug(...interface{})          {}
func (r *warnRecorder) Debugf(string, ...interface{}) {}
func (r *warnRecorder) Info(...interface{})           {}
func (r *warnRecorder) Infof(string, ...interface{})  {}
func (r *warnRecorder) Error(...interface{})          {}
func (r *warnRecorder) Errorf(string, ...interface{}) {}
func (r *warnRecorder) Fatal(...interface{})          {}
func (r *warnRecorder) Fatalf(string, ...interface{}) {}
func (r *warnRecorder) Panic(...interface{})          {}
func (r *warnRecorder) Panicf(string, ...interface{}) {}

func sealingNode(sealer Sealer, logger *warnRecorder) *Node {
	return &Node{config: Config{
		Sealer:        sealer,
		Logger:        logger,
		ExchangeKey:   "member-key",
		SealKeyLength: 43,
		// An interval this long keeps the seal key fixed while a test runs.
		SealKeyInterval: 1 << 30,
	}}
}

// A peer without the network key can publish to the gossip topic. Members
// cannot unseal what it sends, so the handlers never see it, and the warning
// says what went wrong and who sent it.
func TestAMessageThatCannotBeUnsealedIsDropped(t *testing.T) {
	logger := &warnRecorder{}
	n := sealingNode(&crypto.AESSealer{}, logger)

	foreign, err := (&crypto.AESSealer{}).Seal("from a stranger", "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := (&crypto.AESSealer{}).Seal("from a member", n.sealkey())
	if err != nil {
		t.Fatal(err)
	}

	received := make(chan string, 2)
	handler := func(_ *blockchain.Ledger, m *hub.Message, _ chan *hub.Message) error {
		received <- m.Message
		return nil
	}
	roomMessages := make(chan *hub.Message, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go n.handleEvents(ctx, make(chan *hub.Message), roomMessages, func(*hub.Message) error { return nil }, []Handler{handler}, false)

	roomMessages <- &hub.Message{Message: foreign, SenderID: "stranger-peer"}
	roomMessages <- &hub.Message{Message: sealed, SenderID: "member-peer"}

	select {
	case got := <-received:
		if got != "from a member" {
			t.Fatalf("the first message the handlers saw is %q, want only the member's message", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the member's message never reached the handlers")
	}
	select {
	case got := <-received:
		t.Fatalf("the handlers also saw %q", got)
	case <-time.After(100 * time.Millisecond):
	}

	warns := logger.all()
	if len(warns) != 1 {
		t.Fatalf("got warnings %q, want one for the message that could not be unsealed", warns)
	}
	if strings.Contains(warns[0], "%!") || !strings.Contains(warns[0], "stranger-peer") {
		t.Errorf("warning %q is garbled or does not name the sender", warns[0])
	}
}

type failingSealer struct{ crypto.AESSealer }

func (*failingSealer) Seal(string, string) (string, error) {
	return "", errors.New("sealing failed")
}

// A message the node cannot seal is not published at all.
func TestAMessageThatCannotBeSealedIsNotPublished(t *testing.T) {
	logger := &warnRecorder{}
	n := sealingNode(&failingSealer{}, logger)

	published := make(chan *hub.Message, 1)
	input := make(chan *hub.Message, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go n.handleEvents(ctx, input, make(chan *hub.Message), func(m *hub.Message) error {
		published <- m
		return nil
	}, nil, false)

	input <- &hub.Message{Message: "ledger update", SenderID: "this-node"}

	select {
	case m := <-published:
		t.Fatalf("published %+v although sealing failed", m)
	case <-time.After(200 * time.Millisecond):
	}
	warns := logger.all()
	if len(warns) != 1 || strings.Contains(warns[0], "%!") || !strings.Contains(warns[0], "sealing failed") {
		t.Errorf("got warnings %q, want one readable warning with the sealing error", warns)
	}
}
