module github.com/mudler/edgevpn

go 1.14

require (
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/ipfs/go-ipns v0.1.2 // indirect
	github.com/ipfs/go-log/v2 v2.3.0
	github.com/kr/text v0.2.0 // indirect
	github.com/libp2p/go-libp2p v0.15.0
	github.com/libp2p/go-libp2p-core v0.9.0
	github.com/libp2p/go-libp2p-discovery v0.5.1
	github.com/libp2p/go-libp2p-kad-dht v0.11.1
	github.com/libp2p/go-libp2p-pubsub v0.5.4
	github.com/lthibault/jitterbug v2.0.0+incompatible
	github.com/multiformats/go-multiaddr v0.4.0
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pkg/errors v0.9.1
	github.com/songgao/packets v0.0.0-20160404182456-549a10cd4091
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	github.com/vishvananda/netlink v1.1.0
	github.com/xlzd/gotp v0.0.0-20181030022105-c8557ba2c119
	go.uber.org/zap v1.19.0
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	gopkg.in/yaml.v2 v2.4.0
	honnef.co/go/tools v0.1.1 // indirect
)

//replace berty.tech/go-libp2p-tor-transport => github.com/Jorropo/go-libp2p-tor-transport v0.5.2-0.20210219105543-8147363e3140
