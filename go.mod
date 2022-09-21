module github.com/mudler/edgevpn

go 1.16

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/benbjohnson/clock v1.3.0
	github.com/c-robinson/iplib v1.0.3
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/gopacket v1.1.19
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/ipfs/go-log v1.0.5
	github.com/ipfs/go-log/v2 v2.5.1
	github.com/ipld/go-ipld-prime v0.18.0 // indirect
	github.com/labstack/echo/v4 v4.9.0
	github.com/libp2p/go-libp2p v0.23.0
	github.com/libp2p/go-libp2p-kad-dht v0.18.0
	github.com/libp2p/go-libp2p-pubsub v0.8.0
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/miekg/dns v1.1.50
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mudler/go-processmanager v0.0.0-20220724164624-c45b5c61312d
	github.com/multiformats/go-multiaddr v0.7.0
	github.com/onsi/ginkgo/v2 v2.1.1
	github.com/onsi/gomega v1.17.0
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/songgao/packets v0.0.0-20160404182456-549a10cd4091
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	github.com/spf13/cast v1.5.0 // indirect
	github.com/urfave/cli v1.22.10
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20211101163701-50045581ed74 // indirect
	github.com/xlzd/gotp v0.0.0-20220817083547-a63b9d03d72f
	go.uber.org/zap v1.23.0
	golang.org/x/crypto v0.0.0-20220829220503-c86fa9a7ed90 // indirect
	golang.org/x/sync v0.0.0-20220819030929-7fc1605a5dde // indirect
	golang.org/x/sys v0.0.0-20220829200755-d48e67d00261 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/elastic/gosigar => github.com/mudler/gosigar v0.14.3-0.20220502202347-34be910bdaaf
