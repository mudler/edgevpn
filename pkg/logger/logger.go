// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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

package logger

import (
	"fmt"
	"os"

	terminal "github.com/mudler/go-isterminal"

	"github.com/ipfs/go-log"
	"github.com/pterm/pterm"
)

var _ log.StandardLogger = &Logger{}

type Logger struct {
	level log.LogLevel
}

func New(lvl log.LogLevel) *Logger {
	if !terminal.IsTerminal(os.Stdout) {
		pterm.DisableColor()
	}
	if lvl == log.LevelDebug {
		pterm.EnableDebugMessages()
	}
	return &Logger{level: lvl}
}

func joinMsg(args ...interface{}) (message string) {
	for _, m := range args {
		message += " " + fmt.Sprintf("%v", m)
	}
	return
}

func (l Logger) enabled(lvl log.LogLevel) bool {
	return lvl >= l.level
}

func (l Logger) Debug(args ...interface{}) {
	if l.enabled(log.LevelDebug) {
		pterm.Debug.Println(joinMsg(args...))
	}
}

func (l Logger) Debugf(f string, args ...interface{}) {
	if l.enabled(log.LevelDebug) {
		pterm.Debug.Printfln(f, args...)
	}
}

func (l Logger) Error(args ...interface{}) {
	if l.enabled(log.LevelError) {
		pterm.Error.Println(pterm.LightRed(joinMsg(args...)))
	}
}

func (l Logger) Errorf(f string, args ...interface{}) {
	if l.enabled(log.LevelError) {
		pterm.Error.Printfln(pterm.LightRed(f), args...)
	}
}

func (l Logger) Fatal(args ...interface{}) {
	if l.enabled(log.LevelFatal) {
		pterm.Fatal.Println(pterm.Red(joinMsg(args...)))
	}
}

func (l Logger) Fatalf(f string, args ...interface{}) {
	if l.enabled(log.LevelFatal) {
		pterm.Fatal.Printfln(pterm.Red(f), args...)
	}
}

func (l Logger) Info(args ...interface{}) {
	if l.enabled(log.LevelInfo) {
		pterm.Info.Println(pterm.LightBlue(joinMsg(args...)))
	}
}

func (l Logger) Infof(f string, args ...interface{}) {
	if l.enabled(log.LevelInfo) {
		pterm.Info.Printfln(pterm.LightBlue(f), args...)
	}
}

func (l Logger) Panic(args ...interface{}) {
	l.Fatal(args...)
}

func (l Logger) Panicf(f string, args ...interface{}) {
	l.Fatalf(f, args...)
}

func (l Logger) Warn(args ...interface{}) {
	if l.enabled(log.LevelWarn) {
		pterm.Warning.Println(pterm.LightYellow(joinMsg(args...)))
	}
}

func (l Logger) Warnf(f string, args ...interface{}) {
	if l.enabled(log.LevelWarn) {
		pterm.Warning.Printfln(pterm.LightYellow(f), args...)
	}
}

func (l Logger) Warning(args ...interface{}) {
	l.Warn(args...)
}

func (l Logger) Warningf(f string, args ...interface{}) {
	l.Warnf(f, args...)
}
