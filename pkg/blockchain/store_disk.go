package blockchain

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/peterbourgon/diskv"
)

type DiskStore struct {
	chain *diskv.Diskv
}

func NewDiskStore(d *diskv.Diskv) *DiskStore {
	return &DiskStore{chain: d}
}

func (m *DiskStore) Add(b Block) {
	bb, _ := json.Marshal(b)
	m.chain.Write(fmt.Sprint(b.Index), bb)
	m.chain.Write("index", []byte(fmt.Sprint(b.Index)))

}

func (m *DiskStore) Len() int {
	count, err := m.chain.Read("index")
	if err != nil {
		return 0
	}
	c, _ := strconv.Atoi(string(count))
	return c

}

func (m *DiskStore) Last() Block {
	b := &Block{}

	count, err := m.chain.Read("index")
	if err != nil {
		return *b
	}

	dat, _ := m.chain.Read(string(count))
	json.Unmarshal(dat, b)

	return *b
}
