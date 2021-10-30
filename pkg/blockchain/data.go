package blockchain

import "encoding/json"

type Data string

// Unmarshal the result into the interface. Use it to retrieve data
// set with SetValue
func (d Data) Unmarshal(i interface{}) error {
	return json.Unmarshal([]byte(d), i)
}
