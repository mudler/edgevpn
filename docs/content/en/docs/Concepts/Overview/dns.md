---
title: "DNS"
linkTitle: "DNS"
weight: 20
date: 2017-01-05
description: >
  Embedded DNS server documentation
---

## DNS Server

Note: Experimental feature!

A DNS Server is available but disabled by default. 

The DNS server will resolve DNS queries using the blockchain as a record and will forward unknown domains by default.

It can be enabled by specifying a listening address with `--dns`. For example, to bind to default `53` port locally, run in the console:

```bash
edgevpn --dns "127.0.0.1:53"
```

To turn off dns forwarding, specify `--dns-forwarder=false`. Optionally a list of DNS servers can be specified multiple times with `--dns-forward-server`.

The dns subcommand has several options:

```
   --dns value                             DNS listening address. Empty to disable dns server [$DNSADDRESS]
   --dns-forwarder                         Enables dns forwarding [$DNSFORWARD]                 
   --dns-cache-size value                  DNS LRU cache size (default: 200) [$DNSCACHESIZE]                  
   --dns-forward-server value              List of DNS forward server (default: "8.8.8.8:53", "1.1.1.1:53") [$DNSFORWARDSERVER]
```

Nodes of the VPN can start a local DNS server which will resolve the routes stored in the chain.

For example, to add DNS records, use the API as such:

```bash
$ curl -X POST http://localhost:8080/api/dns --header "Content-Type: application/json" -d '{ "Regex": "foo.bar", "Records": { "A": "2.2.2.2" } }'
```

The `/api/dns` routes accepts `POST` requests as `JSON` of the following form:

```json
{ "Regex": "<regex>", 
  "Records": { 
     "A": "2.2.2.2",
     "AAAA": "...",
  },
}
```

Note, `Regex` accepts regexes which will match the DNS requests received and resolved to the specified entries.
