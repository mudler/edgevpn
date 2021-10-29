package cmd

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/edgevpn"
	"github.com/urfave/cli"
	"go.uber.org/zap"
)

func API(l *zap.Logger) cli.Command {
	return cli.Command{
		Name:        "api",
		Description: "api starts an http server to display network informations",
		Flags: append(CommonFlags,
			&cli.StringFlag{
				Name:  "listen",
				Value: ":8080",
			},
		),
		Action: func(c *cli.Context) error {
			e := edgevpn.New(cliToOpts(l, c)...)

			mw, err := e.MessageWriter()
			if err != nil {
				return err
			}

			ledger := blockchain.New(mw, 1000)

			// Join the node to the network, using our ledger
			if err := e.Join(ledger); err != nil {
				return err
			}

			ec := echo.New()

			ec.GET("/api/data", func(c echo.Context) error {
				return c.JSON(http.StatusOK, ledger.CurrentData())
			})

			ec.GET("/api/blockchain", func(c echo.Context) error {
				return c.JSON(http.StatusOK, ledger.BlockChain())
			})

			return ec.Start(c.String("listen"))
		},
	}
}
