package blockchain

type memory struct {
	chain Blockchain
}

func (m *memory) Add(b Block) {
	m.chain = append(m.chain, b)
}

func (m *memory) Reset() {
	m.chain = []Block{}
}

func (m *memory) Len() int {
	return len(m.chain)
}

func (m *memory) Last() Block {
	return m.chain[len(m.chain)-1]
}
