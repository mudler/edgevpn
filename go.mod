module github.com/mudler/edgevpn

go 1.16

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/benbjohnson/clock v1.3.0
	github.com/btcsuite/btcd v0.23.1 // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.2.0 // indirect
	github.com/c-robinson/iplib v1.0.3
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/gopacket v1.1.19
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/ipfs/go-log v1.0.5
	github.com/ipfs/go-log/v2 v2.5.1
	github.com/ipld/go-ipld-prime v0.16.0 // indirect
	github.com/klauspost/compress v1.15.5 // indirect
	github.com/labstack/echo/v4 v4.7.2
	github.com/libp2p/go-libp2p v0.22.0
	github.com/libp2p/go-libp2p-blankhost v0.4.0 // indirect
	github.com/libp2p/go-libp2p-connmgr v0.4.0 // indirect
	github.com/libp2p/go-libp2p-core v0.20.0
	github.com/libp2p/go-libp2p-discovery v0.7.0 // indirect
	github.com/libp2p/go-libp2p-kad-dht v0.17.1-0.20220819144506-26ecb028d38d
	github.com/libp2p/go-libp2p-pubsub v0.6.1
	github.com/libp2p/go-libp2p-resource-manager v0.5.1
	github.com/libp2p/go-libp2p-swarm v0.11.0 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/miekg/dns v1.1.50
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mudler/go-isterminal v0.0.0-20211031135732-5e4e06fc5a58
	github.com/mudler/go-processmanager v0.0.0-20211226182900-899fbb0b97f6
	github.com/multiformats/go-multiaddr v0.6.0
	github.com/onsi/ginkgo/v2 v2.1.1
	github.com/onsi/gomega v1.17.0
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.2 // indirect
	github.com/pterm/pterm v0.12.41
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/songgao/packets v0.0.0-20160404182456-549a10cd4091
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	github.com/spf13/cast v1.5.0 // indirect
	github.com/urfave/cli v1.22.9
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	github.com/xlzd/gotp v0.0.0-20220110052318-fab697c03c2c
	golang.org/x/term v0.0.0-20220526004731-065cf7ba2467 // indirect
	golang.org/x/xerrors v0.0.0-20220609144429-65e65417b02f // indirect
	google.golang.org/grpc v1.47.0 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/elastic/gosigar => github.com/mudler/gosigar v0.14.3-0.20220502202347-34be910bdaaf
