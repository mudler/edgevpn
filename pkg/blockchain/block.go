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

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.Storage = s
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = newBlock.Checksum()

	return newBlock
}
