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

import "testing"

func TestParseOwnershipMode(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want OwnershipMode
	}{
		// The empty string is the zero value of config.Ownership.Mode, i.e. the
		// library default. It must keep meaning "off" or every embedder that
		// never set the field would start failing.
		{"", OwnershipOff},
		{"off", OwnershipOff},
		{"OFF", OwnershipOff},
		{"  off  ", OwnershipOff},
		{"observe", OwnershipObserve},
		{"log", OwnershipObserve},
		{"log-only", OwnershipObserve},
		{"Observe", OwnershipObserve},
		{"enforce", OwnershipEnforce},
		{"on", OwnershipEnforce},
		{"ENFORCE", OwnershipEnforce},
	} {
		got, err := ParseOwnershipMode(tc.in)
		if err != nil {
			t.Errorf("ParseOwnershipMode(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseOwnershipMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// A value we do not recognise must be an error, never a silent fall-through to
// OwnershipOff: that would quietly disable ledger authentication for an
// operator who was trying to turn it on.
func TestParseOwnershipModeRejectsUnknown(t *testing.T) {
	for _, in := range []string{"enabled", "true", "yes", "1", "enforcing", "observer", "no", "disable"} {
		got, err := ParseOwnershipMode(in)
		if err == nil {
			t.Errorf("ParseOwnershipMode(%q) = %v, want an error", in, got)
		}
	}
}

// The error has to tell the operator what to type instead.
func TestParseOwnershipModeErrorNamesAcceptedValues(t *testing.T) {
	_, err := ParseOwnershipMode("enabled")
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"enabled", "off", "observe", "enforce"} {
		if !contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err.Error(), want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestOwnershipModeString(t *testing.T) {
	for _, tc := range []struct {
		in   OwnershipMode
		want string
	}{
		{OwnershipOff, "off"},
		{OwnershipObserve, "observe"},
		{OwnershipEnforce, "enforce"},
	} {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("OwnershipMode(%d).String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}
