package blockchain

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sync"
	"time"

	"github.com/mudler/edgevpn/pkg/hub"

	"github.com/pkg/errors"
)

type Ledger struct {
	sync.Mutex
	blockchain Store

	channel io.Writer

	onDisk bool
}

type Store interface {
	Add(Block)
	Len() int
	Last() Block
}

// New returns a new ledger which writes to the writer
func New(w io.Writer, s Store) *Ledger {
	c := &Ledger{channel: w, blockchain: s}
	c.newGenesis()
	return c
}

func (l *Ledger) newGenesis() {
	t := time.Now()
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), map[string]map[string]Data{}, genesisBlock.Checksum(), ""}
	l.blockchain.Add(genesisBlock)
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
				bytes, err := json.Marshal(l.blockchain.Last())
				if err != nil {
					log.Println(err)
				}
				l.channel.Write(bytes)

				l.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Update the blockchain from a message
func (l *Ledger) Update(h *hub.Message) (err error) {
	//chain := make(Blockchain, 0)
	block := &Block{}

	err = json.Unmarshal([]byte(h.Message), block)
	if err != nil {
		err = errors.Wrap(err, "failed unmarshalling blockchain data")
		return
	}

	l.Lock()
	if block.Index > l.blockchain.Len() || (block.Index == l.blockchain.Len() &&
		block.Hash != l.blockchain.Last().Hash) {
		l.blockchain.Add(*block)
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

// AnnounceDeleteBucket Announce a deletion of a bucket. It stops when the bucket is deleted
func (l *Ledger) AnnounceDeleteBucket(ctx context.Context, interval time.Duration, bucket string) {
	del, cancel := context.WithCancel(ctx)

	l.Announce(del, interval, func() {
		_, exists := l.CurrentData()[bucket]
		if exists {
			l.DeleteBucket(bucket)
		} else {
			cancel()
		}
	})
}

// AnnounceDeleteBucketKey Announce a deletion of a key from a bucket. It stops when the key is deleted
func (l *Ledger) AnnounceDeleteBucketKey(ctx context.Context, interval time.Duration, bucket, key string) {
	del, cancel := context.WithCancel(ctx)

	l.Announce(del, interval, func() {
		_, exists := l.CurrentData()[bucket][key]
		if exists {
			l.Delete(bucket, key)
		} else {
			cancel()
		}
	})
}

// Persist Keeps announcing something into the blockchain until it is reconciled
func (l *Ledger) Persist(ctx context.Context, interval time.Duration, bucket, key string, value interface{}) {
	put, cancel := context.WithCancel(ctx)

	l.Announce(put, interval, func() {
		v, exists := l.CurrentData()[bucket][key]
		realv, _ := json.Marshal(value)
		switch {
		case !exists || string(v) != string(realv):
			l.Add(bucket, map[string]interface{}{key: value})
		case exists && string(v) == string(realv):
			cancel()
		}
	})
}

// GetKey retrieve the current key from the blockchain
func (l *Ledger) GetKey(b, s string) (value Data, exists bool) {
	l.Lock()
	defer l.Unlock()

	if l.blockchain.Len() > 0 {
		last := l.blockchain.Last()
		if _, exists = last.Storage[b]; !exists {
			return
		}
		value, exists = last.Storage[b][s]
		if exists {
			return
		}
	}
	return
}

// Exists returns true if there is one element with a matching value
func (l *Ledger) Exists(b string, f func(Data) bool) (exists bool) {
	l.Lock()
	defer l.Unlock()
	if l.blockchain.Len() > 0 {
		for _, bv := range l.blockchain.Last().Storage[b] {
			if f(bv) {
				exists = true
				return
			}
		}
	}

	return
}

// CurrentData returns the current ledger data (locking)
func (l *Ledger) CurrentData() map[string]map[string]Data {
	l.Lock()
	defer l.Unlock()
	return l.blockchain.Last().Storage
}

// // BlockChain returns the current blockchain (locking)
// func (l *Ledger) BlockChain() Blockchain {
// 	l.Lock()
// 	defer l.Unlock()
// 	return l.blockchain
// }

// Add data to the blockchain
func (l *Ledger) Add(b string, s map[string]interface{}) {
	l.Lock()
	current := l.blockchain.Last().Storage
	for s, k := range s {
		if _, exists := current[b]; !exists {
			current[b] = make(map[string]Data)
		}
		dat, _ := json.Marshal(k)
		current[b][s] = Data(string(dat))
	}
	l.Unlock()
	l.writeData(current)
}

// Delete data from the ledger (locking)
func (l *Ledger) Delete(b string, k string) {
	l.Lock()
	new := make(map[string]map[string]Data)
	for bb, kk := range l.blockchain.Last().Storage {
		if _, exists := new[bb]; !exists {
			new[bb] = make(map[string]Data)
		}
		// Copy all keys/v except b/k
		for kkk, v := range kk {
			if !(bb == b && kkk == k) {
				new[bb][kkk] = v
			}
		}
	}
	l.Unlock()
	l.writeData(new)
}

// DeleteBucket deletes a bucket from the ledger (locking)
func (l *Ledger) DeleteBucket(b string) {
	l.Lock()
	new := make(map[string]map[string]Data)
	for bb, kk := range l.blockchain.Last().Storage {
		// Copy all except the specified bucket
		if bb == b {
			continue
		}
		if _, exists := new[bb]; !exists {
			new[bb] = make(map[string]Data)
		}
		for kkk, v := range kk {
			new[bb][kkk] = v
		}
	}
	l.Unlock()
	l.writeData(new)
}

// String returns the blockchain as string
func (l *Ledger) String() string {
	bytes, _ := json.MarshalIndent(l.blockchain, "", "  ")
	return string(bytes)
}

// Index returns last known blockchain index
func (l *Ledger) Index() int {
	return l.blockchain.Len()
}

func (l *Ledger) writeData(s map[string]map[string]Data) {
	newBlock := l.blockchain.Last().NewBlock(s)

	if newBlock.IsValid(l.blockchain.Last()) {
		l.Lock()
		l.blockchain.Add(newBlock)
		l.Unlock()
	}

	bytes, err := json.Marshal(l.blockchain.Last())
	if err != nil {
		log.Println(err)
	}

	l.channel.Write(bytes)
}
