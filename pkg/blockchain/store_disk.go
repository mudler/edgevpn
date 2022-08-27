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
