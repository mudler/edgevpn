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

package main

//go:generate go run ./api/generate ./api/public/functions.tmpl ./api/public/index.tmpl ./api/public/index.html
import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/mudler/edgevpn/cmd"
	internal "github.com/mudler/edgevpn/internal"
)

func main() {

	app := &cli.App{
		Name:        "edgevpn",
		Version:     internal.Version,
		Authors:     []*cli.Author{{Name: "Ettore Di Giacinto"}},
		Usage:       "edgevpn --config /etc/edgevpn/config.yaml",
		Description: "edgevpn uses libp2p to build an immutable trusted blockchain addressable p2p network",
		Copyright:   cmd.Copyright,
		Flags:       cmd.MainFlags(),
		Commands: []*cli.Command{
			cmd.Start(),
			cmd.API(),
			cmd.ServiceAdd(),
			cmd.ServiceConnect(),
			cmd.FileReceive(),
			cmd.Proxy(),
			cmd.FileSend(),
			cmd.DNS(),
			cmd.Peergate(),
		},

		Action: cmd.Main(),
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
