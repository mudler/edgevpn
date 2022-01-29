---
title: "Token"
linkTitle: "Token"
weight: 3
description: >
  The edgevpn network token
---

A network token represent the network which edgevpn attempts to establish a connection among peers.

A token is created by encoding in base64 a network configuration.

## Generating tokens

To generate a network token, run in the console:

```
edgevpn -b -g
```

This will print out in screen a base64 token which is ready to be shared on nodes that you wish to join on the same network.

## Generating configuration files

EdgeVPN can read both tokens and network configuration files. 

To generate a configuration file, run in the console:

```
edgevpn -g
```

To turn out a config to a token, you must encode in base64:

```
TOKEN=$(edgevpn -g | base64 -w0)
```

which is equivalent to run `edgevpn -g -b`.

## Anatomy of a configuration file

A typical configuration file looks like the following:

```yaml
otp:
  dht:
    interval: 9000
    key: LHKNKT6YZYQGGY3JANGXMLJTHRH7SW3C
    length: 32
  crypto:
    interval: 9000
    key: SGIB6NYJMSRJF2AJDGUI2NDB5LBVCPLS
    length: 32
room: ubONSBFkdWbzkSBTglFzOhWvczTBQJOR
rendezvous: exoHOajMYMSPrHhevAEEjnCHLssFfzfT
mdns: VoZfePlTchbSrdmivaqaOyQyEnTMlugi
max_message_size: 20971520
```

The values can be all tweaked to your needs.

EdgeVPN uses an otp mechanism to decrypt blockchain messages between the nodes and to discover nodes from DHT, this is in order to prevent bruteforce attacks and avoid bad actors listening on the protocol.
See [the Architecture section]() for more information.

- The OTP keys (`otp.crypto.key`) rotates the cipher key used to encode/decode the blockchain messages. The interval of rotation can be set for both DHT and the Blockchain messages. The length is the cipher key length (AES-256 by default) used by the sealer to decrypt/encrypt messages.
- The DHT OTP keys (`otp.dht.key`) rotates the discovery key used during DHT node discovery. A key is generated and used with OTP at defined intervals to scramble potential listeners.
- The `room` is a unique ID which all the nodes will subscribe to. It is automatically generated
- Optionally the OTP mechanism can be disabled by commenting the `otp` block. In this case the static DHT rendezvous will be `rendezvous`
- The `mdns` discovery doesn't have any OTP rotation, so a unique identifier must be provided.
- Here can be defined the max message size accepted for the blockchain messages with `max_message_size` (in bytes)
