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

import "sync"

type MemoryStore struct {
	sync.Mutex
	block *Block
}

func (m *MemoryStore) Add(b Block) {
	m.Lock()
	m.block = &b
	m.Unlock()
}

func (m *MemoryStore) Len() int {
	m.Lock()
	defer m.Unlock()
	if m.block == nil {
		return 0
	}
	return m.block.Index
}

func (m *MemoryStore) Last() Block {
	m.Lock()
	defer m.Unlock()
	return *m.block
}
