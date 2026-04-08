## Initial handshake

Uses Bittorrent tracker protocol (BEP 15)

### Terms
`seeders` - have 100% of the file. Only uploading
`leechers` - incomplete <100% of the file. They are downloading + uploading. contributing pieces they have
`peers` - seeders + leechers

### Key parts

### connect request
- protocol id (constant): fixed 64-bit constant defined by the protocol. Just to let the tracker know that this is a bittorrent UDP tracker req
- action: tell the tracker which action
    - 0 → connect
    - 1 → announce
    - 2 → scrape
    - 3 → error
- transaction id: since there are no gaurantees in UDP, resp may be out of order, use txn_id to match resp with req
- connection id: sent by the tracker, temp session token. expires 1-2~ min, use in announce req

### announce req

**announce req structure (98 bytes)**

```
Offset  Size  Name
0       8     connection_id
8       4     action (1 = announce)
12      4     transaction_id
16      20    info_hash
36      20    peer_id
56      8     downloaded
64      8     left
72      8     uploaded
80      4     event
84      4     IP address (0 = default)
88      4     key (random)
92      4     num_want (-1 = default)
96      2     port
```

### Peer Messaging Procol (PMP)

**First handshake**

```
1 byte   → 19
19 bytes → "BitTorrent protocol"
8 bytes  → reserved (all zeros fine)
20 bytes → info_hash
20 bytes → peer_id
```
