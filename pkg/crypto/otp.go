/*
Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package crypto

import (
	"encoding/base64"
	"hash"

	"github.com/creachadair/otp"
)

func TOTP(f func() hash.Hash, digits int, t int, key string) string {
	return totp(f, digits, otp.TimeWindow(t), key)
}

// TOTPAt returns the password TOTP gives during the time window with the
// given number, where window n covers seconds [n*t, (n+1)*t) of Unix time for
// an interval of t seconds.
func TOTPAt(f func() hash.Hash, digits int, window uint64, key string) string {
	return totp(f, digits, func() uint64 { return window }, key)
}

func totp(f func() hash.Hash, digits int, step func() uint64, key string) string {
	cfg := otp.Config{
		Hash:     f,      // default is sha1.New
		Digits:   digits, // default is 6
		TimeStep: step,
		Key:      key,
		Format: func(hash []byte, nb int) string {
			return base64.StdEncoding.EncodeToString(hash)[:nb]
		},
	}
	return cfg.TOTP()
}
