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

package hub

import "encoding/json"

// Message gets converted to/from JSON and sent in the body of pubsub messages.
type Message struct {
	Message  string
	SenderID string

	Annotations map[string]interface{}
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

func (m *Message) Copy() *Message {
	copy := *m
	return &copy
}

func (m *Message) WithMessage(s string) *Message {
	copy := m.Copy()
	copy.Message = s
	return copy
}

func (m *Message) AnnotationsToObj(v interface{}) error {
	blob, err := json.Marshal(m.Annotations)
	if err != nil {
		return err
	}
	return json.Unmarshal(blob, v)
}
