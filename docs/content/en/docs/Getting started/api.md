---
title: "WebUI and API"
linkTitle: "WebUI and API"
weight: 1
description: >
  Query the network status and operate the ledger with the built-in API
---

The API has a simple webUI embedded to display network informations.


To access the web interface, run in the console:

```bash
$ edgevpn api
```

with either a `EDGEVPNCONFIG` or `EDGEVPNTOKEN`. 

Dashboard (Dark mode)            |  Dashboard (Light mode)
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 00-12-16 EdgeVPN - Machines index](https://user-images.githubusercontent.com/2420543/163020448-8e9238c1-3b6d-435d-9b25-7729d8779ebd.png) | ![Screenshot 2021-10-31 at 23-03-26 EdgeVPN - Machines index](https://user-images.githubusercontent.com/2420543/163020460-e18c07d7-8426-4992-aab3-0b2fd90279ae.png)

DNS            |  Machine index
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 23-03-44 EdgeVPN - Services index](https://user-images.githubusercontent.com/2420543/163020465-3d481da4-4912-445e-afc0-2614966dcadf.png) | ![Screenshot 2021-10-31 at 23-03-59 EdgeVPN - Files index](https://user-images.githubusercontent.com/2420543/163020462-7821a622-8c13-4971-8abe-9c5b6b491ae8.png)

Services            |  Blockchain index
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 23-04-12 EdgeVPN - Users connected](https://user-images.githubusercontent.com/2420543/163021285-3c5a980d-2562-4c10-b266-7e99f19d8a87.png) | ![Screenshot 2021-10-31 at 23-04-20 EdgeVPN - Blockchain index](https://user-images.githubusercontent.com/2420543/163020457-77ef6e50-40a6-4e3b-83c4-a81db729bd7d.png)


In API mode, EdgeVPN will connect to the network without routing any packet, and without setting up a VPN interface. 

By default edgevpn will listen on the `8080` port. See `edgevpn api --help` for the available options

API can also be started together with the vpn with `--api`.

## API endpoints

### GET

#### `/api/users`

Returns the users connected to services in the blockchain

#### `/api/services`

Returns the services running in the blockchain

#### `/api/dns`

Returns the domains registered in the blockchain

#### `/api/machines`

Returns the machines connected to the VPN

#### `/api/blockchain`

Returns the latest available blockchain

#### `/api/ledger`

Returns the current data in the ledger

#### `/api/ledger/:bucket`

Returns the current data in the ledger inside the `:bucket`

#### `/api/ledger/:bucket/:key`

Returns the current data in the ledger inside the `:bucket` at given `:key`

#### `/api/peergate`

Returns peergater status

### PUT

#### `/api/ledger/:bucket/:key/:value`

Puts `:value` in the ledger inside the `:bucket` at given `:key`

#### `/api/peergate/:state`

Enables/disables peergating:

```bash
# enable
$ curl -X PUT 'http://localhost:8080/api/peergate/enable'
# disable
$ curl -X PUT 'http://localhost:8080/api/peergate/disable'
```

### POST

#### `/api/dns`

The endpoint accept a JSON payload of the following form:

```json
{ "Regex": "<regex>", 
  "Records": { 
     "A": "2.2.2.2",
     "AAAA": "...",
  },
}
```

Takes a regex and a set of records and registers them to the blockchain.

The DNS table in the ledger will be used by the embedded DNS server to handle requests locally.

To create a new entry, for example:

```bash
$ curl -X POST http://localhost:8080/api/dns --header "Content-Type: application/json" -d '{ "Regex": "foo.bar", "Records": { "A": "2.2.2.2" } }'
```

### DELETE

#### `/api/ledger/:bucket/:key`

Deletes the `:key` into `:bucket` inside the ledger

#### `/api/ledger/:bucket`

Deletes the `:bucket` from the ledger

## Binding to a socket

The API can also be bound to a socket, for instance:

```bash
$ edgevpn api --listen "unix://<path/to/socket>"
```

or as well while running the vpn:

```bash
$ edgevpn api --api-listen "unix://<path/to/socket>"
```
