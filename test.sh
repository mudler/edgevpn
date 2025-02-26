#!/bin/bash

keys=($(go run main.go peergater ecdsa-genkey | cut -d: -f2- ))
PRIVKEY=${keys[0]}
PUBKEY=${keys[1]}

# export EDGEVPNDHTANNOUNCEMADDRS=/ip4/.../tcp/.../p2p/...
export EDGEVPNCONFIG=~/config.yml
export EDGEVPNPEERGATEINTERVAL=10
export PEERGATE=true
export PEERGUARD=true
export PEERGATE_AUTOCLEAN=true
export PEERGATE_AUTH='{ "ecdsa" : { "private_key": "'$PRIVKEY'" } }'
export PEERGATE_PUBLIC='{ "ecdsa_1": "'$PUBKEY'" }'

# killall main is a bad idea, but that worked on my machine
sudo -E bash -c "
  IFACE=\"utun10\" go run main.go api &
  IFACE=\"utun11\" go run main.go
  killall main
"
