---
title: "webUI and API"
linkTitle: "webUI and API"
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


Dashboard            |  Machine index
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 00-12-16 EdgeVPN - Machines index](https://user-images.githubusercontent.com/2420543/139602703-f04ac4cb-b949-498c-a23a-0ce8deb036f9.png) | ![Screenshot 2021-10-31 at 23-03-26 EdgeVPN - Machines index](https://user-images.githubusercontent.com/2420543/139602704-15bd342f-2db2-4a3b-b1c7-4dc7be27c0f4.png)

Services            |  File index
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 23-03-44 EdgeVPN - Services index](https://user-images.githubusercontent.com/2420543/139602706-6050dfb7-2ef1-45b2-a768-a00b9de60ba1.png) | ![Screenshot 2021-10-31 at 23-03-59 EdgeVPN - Files index](https://user-images.githubusercontent.com/2420543/139602707-1d29f9b4-972c-490f-8015-067fbf5580f2.png)

Users            |  Blockchain index
:-------------------------:|:-------------------------:
![Screenshot 2021-10-31 at 23-04-12 EdgeVPN - Users connected](https://user-images.githubusercontent.com/2420543/139602708-d102ae09-12f2-4c4c-bcc2-d8f4366355e0.png) | ![Screenshot 2021-10-31 at 23-04-20 EdgeVPN - Blockchain index](https://user-images.githubusercontent.com/2420543/139602709-244960bb-ea1d-413b-8c3e-8959133427ae.png)



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

### PUT

#### `/api/ledger/:bucket/:key/:value`

Puts `:value` in the ledger inside the `:bucket` at given `:key`

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

Takes a regex and a set of records and registers then to the blockchain.

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
