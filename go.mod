module github.com/mudler/edgevpn

go 1.16

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/benbjohnson/clock v1.3.0
	github.com/c-robinson/iplib v1.0.3
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/gookit/color v1.5.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4
	github.com/ipfs/go-log v1.0.5
	github.com/ipfs/go-log/v2 v2.5.1
	github.com/ipld/go-ipld-prime v0.14.4 // indirect
	github.com/labstack/echo/v4 v4.6.3
	github.com/libp2p/go-libp2p v0.19.1
	github.com/libp2p/go-libp2p-connmgr v0.3.1
	github.com/libp2p/go-libp2p-core v0.15.1
	github.com/libp2p/go-libp2p-discovery v0.6.0
	github.com/libp2p/go-libp2p-kad-dht v0.15.0
	github.com/libp2p/go-libp2p-mplex v0.6.0
	github.com/libp2p/go-libp2p-pubsub v0.6.1
	github.com/libp2p/go-libp2p-resource-manager v0.2.1
	github.com/libp2p/go-libp2p-yamux v0.9.1
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/miekg/dns v1.1.48
	github.com/mudler/go-isterminal v0.0.0-20211031135732-5e4e06fc5a58
	github.com/mudler/go-processmanager v0.0.0-20211226182900-899fbb0b97f6
	github.com/multiformats/go-multiaddr v0.5.0
	github.com/onsi/ginkgo/v2 v2.1.1
	github.com/onsi/gomega v1.17.0
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pterm/pterm v0.12.36
	github.com/songgao/packets v0.0.0-20160404182456-549a10cd4091
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	github.com/urfave/cli v1.22.5
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	github.com/xlzd/gotp v0.0.0-20220110052318-fab697c03c2c
	golang.org/x/net v0.0.0-20220418201149-a630d4f3e7a2
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/elastic/gosigar => github.com/mudler/gosigar v0.14.3-0.20220502202347-34be910bdaaf
