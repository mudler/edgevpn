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
	"fmt"
	"strings"
)

// String renders the mode the way an operator writes it on the command line.
func (m OwnershipMode) String() string {
	switch m {
	case OwnershipObserve:
		return "observe"
	case OwnershipEnforce:
		return "enforce"
	default:
		return "off"
	}
}

// ParseOwnershipMode maps a configuration string to an OwnershipMode. It is the
// single source of truth for the accepted spellings — the config layer, the CLI
// and the documentation all go through it, so the accepted set cannot drift
// between them.
//
// An unrecognised value is an error rather than a fall-through to
// OwnershipOff. Falling through silently disabled ledger authentication for
// anyone who wrote --ownership=enabled or misspelled the mode: the node came up
// happily, logged nothing, and accepted forged writes from any token holder.
//
// The empty string is deliberately valid and means off. It is the zero value of
// config.Ownership.Mode, i.e. the library default for embedders that never set
// the field; rejecting it would break them.
func ParseOwnershipMode(s string) (OwnershipMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off":
		return OwnershipOff, nil
	case "observe", "log", "log-only":
		return OwnershipObserve, nil
	case "enforce", "on":
		return OwnershipEnforce, nil
	default:
		return OwnershipOff, fmt.Errorf(
			"invalid ownership mode %q: must be one of %q (legacy, no authentication), %q (sign and log violations; aliases %q, %q) or %q (sign and reject unauthorized writes; alias %q)",
			s, "off", "observe", "log", "log-only", "enforce", "on")
	}
}
