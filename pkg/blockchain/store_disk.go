package blockchain

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/peterbourgon/diskv"
)

type disk struct {
	chain *diskv.Diskv
}

func (m *disk) Add(b Block) {
	bb, _ := json.Marshal(b)
	m.chain.Write(fmt.Sprint(b.Index), bb)
	m.chain.Write("index", []byte(fmt.Sprint(b.Index)))

}

func (m *disk) Reset() {
	m.chain.EraseAll()
}

func (m *disk) Len() int {
	count, err := m.chain.Read("index")
	if err != nil {
		return 0
	}
	c, _ := strconv.Atoi(string(count))
	return c

}

func (m *disk) Last() Block {
	b := &Block{}

	count, err := m.chain.Read("index")
	if err != nil {
		return *b
	}

	dat, _ := m.chain.Read(string(count))
	json.Unmarshal(dat, b)

	return *b
}
