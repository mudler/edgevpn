package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Block represents each 'item' in the blockchain
type Block struct {
	Index      int
	Timestamp  string
	AddressMap map[string]string
	Hash       string
	PrevHash   string
}

// Blockchain is a series of validated Blocks
type Blockchain []Block

func (b Blockchain) IsMoreRecent(bb Blockchain) bool {
	return len(b) > len(bb) || len(b) == len(bb) && b[len(b)-1].Hash != bb[len(bb)-1].Hash
}

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
	record := fmt.Sprint(b.Index, b.Timestamp, b.AddressMap, b.PrevHash)
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// create a new block using previous block's hash
func (oldBlock Block) NewBlock(s map[string]string) Block {

	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.AddressMap = s
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = newBlock.Checksum()

	return newBlock
}
