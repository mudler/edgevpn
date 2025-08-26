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

package blockchain

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/mudler/edgevpn/pkg/hub"
	"github.com/mudler/edgevpn/pkg/utils"

	"github.com/pkg/errors"
)

type Ledger struct {
	sync.Mutex
	blockchain Store

	channel io.Writer

	skipVerify         bool
	trustedPeerIDS     []string
	protectedStoreKeys []string
}

type Store interface {
	Add(Block)
	Len() int
	Last() Block
}

// New returns a new ledger which writes to the writer
func New(w io.Writer, s Store) *Ledger {
	c := &Ledger{channel: w, blockchain: s}
	if s.Len() == 0 {
		c.newGenesis()
	}
	return c
}

func (l *Ledger) newGenesis() {
	t := time.Now()
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), map[string]map[string]Data{}, genesisBlock.Checksum(), ""}
	l.blockchain.Add(genesisBlock)
}

func (l *Ledger) SkipVerify() {
	l.skipVerify = true
}
func (l *Ledger) SetTrustedPeerIDS(ids []string) {
	l.trustedPeerIDS = ids
}
func (l *Ledger) SetProtectedStoreKeys(keys []string) {
	l.protectedStoreKeys = keys
}

// Syncronizer starts a goroutine which
// writes the blockchain to the  periodically
func (l *Ledger) Syncronizer(ctx context.Context, t time.Duration) {
	go func() {
		t := utils.NewBackoffTicker(utils.BackoffMaxInterval(t))
		defer t.Stop()
		for {
			select {
			case <-t.C:
				l.Lock()

				bytes, err := json.Marshal(l.blockchain.Last())
				if err != nil {
					log.Println(err)
				}

				l.channel.Write(compress(bytes).Bytes())

				l.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func compress(b []byte) *bytes.Buffer {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(b)
	gz.Close()
	return &buf
}

func deCompress(b []byte) (*bytes.Buffer, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	result, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(result), nil
}

// Update the blockchain from a message
func (l *Ledger) Update(f *Ledger, h *hub.Message, c chan *hub.Message) (err error) {
	//chain := make(Blockchain, 0)
	block := &Block{}

	b, err := deCompress([]byte(h.Message))
	if err != nil {
		err = errors.Wrap(err, "failed decompressing")
		return
	}

	err = json.Unmarshal(b.Bytes(), block)
	if err != nil {
		err = errors.Wrap(err, "failed unmarshalling blockchain data")
		return
	}

	if len(l.protectedStoreKeys) > 0 && !slices.Contains(l.trustedPeerIDS, h.SenderID) {
		for _, key := range l.protectedStoreKeys {
			if !maps.Equal(l.blockchain.Last().Storage[key], block.Storage[key]) {
				err = errors.Wrapf(err, "unauthorized attempt to write to protected bucket: %s", key)
				return
			}
		}
	}

	l.Lock()
	if l.skipVerify || block.Index > l.blockchain.Len() {
		l.blockchain.Add(*block)
	}
	l.Unlock()

	return
}

// Announce keeps updating async data to the blockchain.
// Sends a broadcast at the specified interval
// by making sure the async retrieved value is written to the
// blockchain
func (l *Ledger) Announce(ctx context.Context, d time.Duration, async func()) {
	go func() {
		//t := time.NewTicker(t)
		t := utils.NewBackoffTicker(utils.BackoffMaxInterval(d))
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
// It takes an interval time and a max timeout.
// It is best effort, and the timeout is necessary, or we might flood network with requests
// if more writers are attempting to write to the same resource
func (l *Ledger) AnnounceDeleteBucket(ctx context.Context, interval, timeout time.Duration, bucket string) {
	del, cancel := context.WithTimeout(ctx, timeout)

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
func (l *Ledger) AnnounceDeleteBucketKey(ctx context.Context, interval, timeout time.Duration, bucket, key string) {
	del, cancel := context.WithTimeout(ctx, timeout)

	l.Announce(del, interval, func() {
		_, exists := l.CurrentData()[bucket][key]
		if exists {
			l.Delete(bucket, key)
		} else {
			cancel()
		}
	})
}

// AnnounceUpdate Keeps announcing something into the blockchain if state is differing
func (l *Ledger) AnnounceUpdate(ctx context.Context, interval time.Duration, bucket, key string, value interface{}) {
	l.Announce(ctx, interval, func() {
		v, exists := l.CurrentData()[bucket][key]
		realv, _ := json.Marshal(value)
		switch {
		case !exists || string(v) != string(realv):
			l.Add(bucket, map[string]interface{}{key: value})
		}
	})
}

// Persist Keeps announcing something into the blockchain until it is reconciled
func (l *Ledger) Persist(ctx context.Context, interval, timeout time.Duration, bucket, key string, value interface{}) {
	put, cancel := context.WithTimeout(ctx, timeout)

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

	return buckets(l.blockchain.Last().Storage).copy()
}

// LastBlock returns the last block in the blockchain
func (l *Ledger) LastBlock() Block {
	l.Lock()
	defer l.Unlock()
	return l.blockchain.Last()
}

type bucket map[string]Data

func (b bucket) copy() map[string]Data {
	copy := map[string]Data{}
	for k, v := range b {
		copy[k] = v
	}
	return copy
}

type buckets map[string]map[string]Data

func (b buckets) copy() map[string]map[string]Data {
	copy := map[string]map[string]Data{}
	for k, v := range b {
		copy[k] = bucket(v).copy()
	}
	return copy
}

// Add data to the blockchain
func (l *Ledger) Add(b string, s map[string]interface{}) {
	l.Lock()
	current := buckets(l.blockchain.Last().Storage).copy()

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

	if !l.skipVerify && !newBlock.IsValid(l.blockchain.Last()) {
		return
	}

	l.Lock()
	l.blockchain.Add(newBlock)
	l.Unlock()

	bytes, err := json.Marshal(l.blockchain.Last())
	if err != nil {
		log.Println(err)
	}

	l.channel.Write(compress(bytes).Bytes())
}
