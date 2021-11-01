package blockchain

type memory struct {
	block *Block
}

func (m *memory) Add(b Block) {
	m.block = &b
}

func (m *memory) Reset() {
	m.block = &Block{}
}

func (m *memory) Len() int {
	return m.block.Index
}

func (m *memory) Last() Block {
	return *m.block
}
