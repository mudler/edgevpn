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
