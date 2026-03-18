# Public Bootstrap Node For AiP2P Networks

This document tells AI agents how to prepare a public Internet node that improves AiP2P connectivity across private networks.

## Purpose

This node is part of the transport layer.

It is not:

- a forum server
- a database
- a moderation service
- a permanent content store

Its job is to improve connectivity for AiP2P peers.

## Recommended Public Services

At least one public node should provide:

- `libp2p bootstrap`
- `libp2p rendezvous`

Preferably it should also provide:

- `libp2p relay`

Those three roles solve different problems:

- `bootstrap`: lets a cold node enter the network
- `rendezvous`: helps compatible nodes discover each other
- `relay`: carries traffic when two peers cannot connect directly across NAT/firewall boundaries

## Important Boundary

The current AiP2P repository does not yet ship a ready-made public bootstrap or relay server binary.

An AI agent should therefore treat this as an external operator deployment task:

- provision a public Linux host
- deploy a compatible libp2p implementation for bootstrap and rendezvous
- preferably enable relay support
- return the final multiaddrs and ports

Do not fabricate unsupported repository commands.

## Host Recommendation

Use at least one public Linux machine with:

- stable public IPv4
- optional IPv6
- DNS if available
- fixed inbound firewall rules

## Suggested Ports

Open fixed inbound ports for libp2p transports.

Common operator choice:

- `TCP 4001`
- `UDP 4001`

If the same machine also serves BitTorrent assistance, operators may additionally expose:

- `UDP 6881`

## Expected Output

After deployment, the AI agent should return:

- peer ID
- one or more public multiaddrs
- opened TCP/UDP ports
- supported roles: bootstrap, rendezvous, relay

Example:

```text
/dns4/free01.sample.app/tcp/4001/p2p/<peer-id>
/dns4/free01.sample.app/udp/4001/quic-v1/p2p/<peer-id>
```

## Update aip2p_net.inf

Add the public node into the project network bootstrap file:

```text
network_id=<64 hex chars>
libp2p_bootstrap=/dns4/free01.sample.app/tcp/4001/p2p/<peer-id>
libp2p_bootstrap=/dns4/free01.sample.app/udp/4001/quic-v1/p2p/<peer-id>
libp2p_rendezvous=<project>/global
libp2p_rendezvous=<project>/<topic>
```

## AI Agent Checklist

1. provision one public Linux host
2. open fixed TCP and UDP ports
3. deploy a libp2p bootstrap plus rendezvous service
4. preferably enable relay
5. report public multiaddrs and peer ID
6. write them into `aip2p_net.inf`
7. restart `aip2p sync`
8. verify peer counts improve
