package blockchain

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sync"
	"time"

	"github.com/mudler/edgevpn/pkg/hub"
)

type Ledger struct {
	sync.Mutex
	Blockchain Blockchain

	maxChainSize int
	channel      io.Writer
}

// New returns a new ledger which writes to the writer
func New(w io.Writer, maxChainSize int) *Ledger {
	c := &Ledger{channel: w, maxChainSize: maxChainSize}
	c.newGenesis()
	return c
}

func (l *Ledger) newGenesis() {
	t := time.Now()
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), map[string]Data{}, genesisBlock.Checksum(), ""}
	l.Blockchain = append(l.Blockchain, genesisBlock)
}

// Syncronizer starts a goroutine which
// writes the blockchain to the  periodically
func (l *Ledger) Syncronizer(ctx context.Context, t time.Duration) {
	go func() {
		t := time.NewTicker(t)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				l.Lock()
				bytes, err := json.Marshal(l.Blockchain)
				if err != nil {
					log.Println(err)
				}
				l.channel.Write(bytes)

				// Reset blockchain if we exceed chainsize
				if l.maxChainSize != 0 && len(l.Blockchain) > l.maxChainSize {
					l.Blockchain = []Block{}
				}
				l.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// String returns the blockchain as string
func (l *Ledger) String() string {
	bytes, _ := json.MarshalIndent(l.Blockchain, "", "  ")
	return string(bytes)
}

// Update the blockchain from a message
func (l *Ledger) Update(h *hub.Message) (err error) {
	chain := make(Blockchain, 0)

	err = json.Unmarshal([]byte(h.Message), &chain)
	if err != nil {
		return
	}

	l.Lock()
	if chain.IsMoreRecent(l.Blockchain) {
		l.Blockchain = chain
	}
	l.Unlock()

	return
}

// Announce keeps updating async data to the blockchain.
// Sends a broadcast at the specified interval
// by making sure the async retrieved value is written to the
// blockchain
func (l *Ledger) Announce(ctx context.Context, t time.Duration, async func()) {
	go func() {
		t := time.NewTicker(t)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				async()

			case <-ctx.Done():
				return
			}
		}
	}()
}

func (l *Ledger) lastBlock() Block {
	return (l.Blockchain[len(l.Blockchain)-1])
}

// GetKey retrieve the current key from the blockchain
func (l *Ledger) GetKey(s string) (value Data, exists bool) {
	l.Lock()
	defer l.Unlock()

	if len(l.Blockchain) > 0 {
		last := l.lastBlock()
		value, exists = last.Storage[s]
		if exists {
			return
		}
	}

	return
}

// ExistsValue returns true if there is one element with a matching value
func (l *Ledger) Exists(f func(Data) bool) (exists bool) {
	l.Lock()
	defer l.Unlock()
	if len(l.Blockchain) > 0 {
		for _, bv := range l.lastBlock().Storage {
			if f(bv) {
				exists = true
				return
			}
		}
	}

	return
}

// Add data to the blockchain
func (l *Ledger) Add(s map[string]Data) {
	l.Lock()
	current := l.lastBlock().Storage
	for s, k := range s {
		current[s] = k
	}
	l.Unlock()
	l.writeData(current)
}

func (l *Ledger) writeData(s map[string]Data) {
	newBlock := l.lastBlock().NewBlock(s)

	if newBlock.IsValid(l.lastBlock()) {
		l.Lock()
		l.Blockchain = append(l.Blockchain, newBlock)
		l.Unlock()
	}

	bytes, err := json.Marshal(l.Blockchain)
	if err != nil {
		log.Println(err)
	}

	l.channel.Write(bytes)
}
