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

package config

import "testing"

func ownershipConfig(mode string) Config {
	return Config{NetworkToken: "dummy", Ownership: Ownership{Mode: mode}}
}

// An unrecognised --ownership value used to fall through to "off", silently
// disabling ledger authentication. It must now fail validation, which runs
// before ToOpts builds anything and before the node starts.
func TestValidateRejectsUnknownOwnershipMode(t *testing.T) {
	for _, mode := range []string{"enabled", "true", "yes", "enforcing"} {
		if err := ownershipConfig(mode).Validate(); err == nil {
			t.Errorf("Validate() accepted ownership mode %q, want an error", mode)
		}
	}
}

func TestValidateAcceptsKnownOwnershipModes(t *testing.T) {
	for _, mode := range []string{"", "off", "observe", "log", "log-only", "enforce", "on", "ENFORCE"} {
		if err := ownershipConfig(mode).Validate(); err != nil {
			t.Errorf("Validate() rejected ownership mode %q: %v", mode, err)
		}
	}
}

// ToOpts must surface the same failure, so an embedder cannot start a node with
// ownership silently off.
func TestToOptsRejectsUnknownOwnershipMode(t *testing.T) {
	_, _, err := ownershipConfig("enabled").ToOpts(nil)
	if err == nil {
		t.Fatal("ToOpts() accepted an unknown ownership mode, want an error")
	}
}
