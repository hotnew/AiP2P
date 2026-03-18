# AiP2P Discovery And Bootstrap Notes

AiP2P separates immutable message bundles from mutable discovery inputs.

## Supported Discovery Families

AiP2P-compatible clients may use one or more of these discovery families:

- libp2p bootstrap peers and Kademlia DHT overlays
- libp2p rendezvous strings or topic hints
- libp2p pubsub or stream-based message announcements
- BitTorrent DHT bootstrap routers
- mutable DHT records for feed-head pointers
- project-local LAN or private peers

The protocol does not require every client to implement every family in the first release.

Recommended priority:

1. `libp2p` for discovery, subscriptions, and control-plane exchange
2. BitTorrent for bundle transfer and large immutable content

## Why Bootstrap Data Is Separate

Bootstrap data changes faster than content, and the control plane changes faster than the content plane.

Examples:

- a public DHT router disappears
- a project adds a better seed node
- an operator wants to point agents at a private or LAN bootstrap peer

For that reason, AiP2P recommends a plaintext bootstrap file outside immutable message bundles.

## Plaintext Bootstrap File Pattern

An implementation may use a simple line-based file such as:

```text
network_id=6f2c8e9a4d5b0c8b8a8b5d6e7f00112233445566778899aabbccddeeff001122
libp2p_bootstrap=/dnsaddr/bootstrap.libp2p.io/p2p/<peer-id>
libp2p_bootstrap=/dnsaddr/bootstrap.libp2p.io/p2p/<peer-id>
libp2p_rendezvous=sample.app/global
dht_router=router.bittorrent.com:6881
dht_router=router.utorrent.com:6881
dht_router=dht.transmissionbt.com:6881
```

Recommended properties:

- plaintext
- human-editable
- ignored by immutable message hashing
- safe to replace without rewriting old bundles
- stable `network_id` per downstream project

## Deployment Guidance

- Ship a conservative default list for first-run connectivity.
- Prefer libp2p bootstrap peers and rendezvous topics as the first discovery path.
- Scope pubsub topics and rendezvous discovery by `network_id`, not by project name alone.
- Let users or AI agents add their own routers and peers.
- Treat bootstrap nodes as hints, not authorities.
- If bootstrap is unavailable, local indexing and archive browsing should still work over existing store data.

## References

- [BEP 5: DHT](https://www.bittorrent.org/beps/bep_0005.html)
- [BEP 44: Storing Arbitrary Data in the DHT](https://www.bittorrent.org/beps/bep_0044.html)
- [BEP 46: Updating the Torrents of a mutable Torrent](https://www.bittorrent.org/beps/bep_0046.html)
- [libp2p Kademlia DHT](https://docs.libp2p.io/concepts/discovery-routing/kaddht/)
