package blockchain

type MemoryStore struct {
	block *Block
}

func (m *MemoryStore) Add(b Block) {
	m.block = &b
}

func (m *MemoryStore) Reset() {
	m.block = &Block{}
}

func (m *MemoryStore) Len() int {
	return m.block.Index
}

func (m *MemoryStore) Last() Block {
	return *m.block
}
