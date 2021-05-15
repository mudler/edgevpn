package hub

import (
	"github.com/mudler/edgevpn/pkg/utils"
	"github.com/pkg/errors"
)

// Message gets converted to/from JSON and sent in the body of pubsub messages.
type Message struct {
	Message  string
	SenderID string

	Annotations map[string]string
}

type MessageOption func(cfg *Message) error

// Apply applies the given options to the config, returning the first error
// encountered (if any).
func (m *Message) Apply(opts ...MessageOption) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(m); err != nil {
			return err
		}
	}
	return nil
}

func NewMessage(s string) *Message {
	return &Message{Message: s}
}

func (m *Message) Seal(key string) error {
	enckey := [32]byte{}
	copy(enckey[:], key)
	enc, err := utils.AESEncrypt(m.Message, &enckey)
	if err != nil {
		return errors.Wrap(err, "while sealing message")
	}
	m.Message = enc
	return nil
}

func (m *Message) Unseal(key string) error {
	enckey := [32]byte{}
	copy(enckey[:], key)
	dec, err := utils.AESDecrypt(m.Message, &enckey)
	if err != nil {
		return errors.Wrapf(err, "while unsealing message from peer: %s", m.SenderID)
	}
	m.Message = dec
	return nil
}

func (m *Message) Copy() *Message {
	copy := *m
	return &copy
}

func (m *Message) WithMessage(s string) *Message {
	copy := m.Copy()
	copy.Message = s
	return copy
}
