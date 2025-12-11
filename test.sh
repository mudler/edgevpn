#!/bin/bash

main_keys=($(go run main.go peergater ecdsa-genkey | cut -d: -f2- ))
client_keys=($(go run main.go peergater ecdsa-genkey | cut -d: -f2- ))
MAIN_PRIVKEY=${main_keys[0]}
MAIN_PUBKEY=${main_keys[1]}
CLIENT_PRIVKEY=${client_keys[0]}
CLIENT_PUBKEY=${client_keys[1]}

# export EDGEVPNDHTANNOUNCEMADDRS=/ip4/.../tcp/.../p2p/...
export EDGEVPNCONFIG=config.yml
export EDGEVPNPEERGATEINTERVAL=10
export EDGEVPNPRIVKEY='CAESQOV82ydHYcTFqyjf6fE6Zrdr9aH97GwGODEWm9HmELv73T55KPBrW5n3D29Df7b+DjH1zVzqUa1cgpTBHiEBdgk='
export PEERGATE=true
export PEERGUARD=true
export PEERGATE_AUTOCLEAN=true
export PEERGATE_AUTH='{ "ecdsa" : { "private_key": "'$MAIN_PRIVKEY'" } }'

# killall main is a bad idea, but that worked on my machine
sudo -E bash -c "
  IFACE=\"utun10\" go run main.go api --enable-healthchecks &
  sleep 3

  curl -X PUT http://127.0.0.1:8080/api/ledger/trustzoneAuth/ecdsa_client/"$CLIENT_PUBKEY"

  export -n EDGEVPNPRIVKEY
  export -n PEERGATE
  export PEERGATE_AUTH='{ \"ecdsa\" : { \"private_key\": \""$CLIENT_PRIVKEY"\" } }'
  
  IFACE=\"utun11\" go run main.go
  killall main
"
