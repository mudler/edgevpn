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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type DataString string

// Block represents each 'item' in the blockchain
type Block struct {
	Index     int
	Timestamp string
	Storage   map[string]map[string]Data
	Hash      string
	PrevHash  string
}

// Blockchain is a series of validated Blocks
type Blockchain []Block

// make sure block is valid by checking index, and comparing the hash of the previous block
func (newBlock Block) IsValid(oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	if newBlock.Checksum() != newBlock.Hash {
		return false
	}

	return true
}

// Checksum does SHA256 hashing of the block
func (b Block) Checksum() string {
	record := fmt.Sprint(b.Index, b.Timestamp, b.Storage, b.PrevHash)
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// create a new block using previous block's hash
func (oldBlock Block) NewBlock(s map[string]map[string]Data) Block {
	var newBlock Block

	t := time.Now().UTC()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.Storage = s
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = newBlock.Checksum()

	return newBlock
}
