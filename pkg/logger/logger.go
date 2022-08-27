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
