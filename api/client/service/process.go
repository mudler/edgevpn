// Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package service

import (
	"os/exec"
	"path/filepath"

	process "github.com/mudler/go-processmanager"
)

// NewProcessController returns a new process controller associated with the state directory
func NewProcessController(statedir string) *ProcessController {
	return &ProcessController{stateDir: statedir}
}

// ProcessController syntax sugar around go-processmanager
type ProcessController struct {
	stateDir string
}

// Process returns a process associated within binaries inside the state dir
func (a *ProcessController) Process(state, p string, args ...string) *process.Process {
	return process.New(
		process.WithName(a.BinaryPath(p)),
		process.WithArgs(args...),
		process.WithStateDir(filepath.Join(a.stateDir, "proc", state)),
	)
}

// BinaryPath returns the binary path of the program requested as argument.
// The binary path is relative to the process state directory
func (a *ProcessController) BinaryPath(b string) string {
	return filepath.Join(a.stateDir, "bin", b)
}

// Run simply runs a command from a binary in the state directory
func (a *ProcessController) Run(command string, args ...string) (string, error) {
	cmd := exec.Command(a.BinaryPath(command), args...)
	out, err := cmd.CombinedOutput()

	return string(out), err
}
