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

From client
```
1 byte   → 19
19 bytes → "BitTorrent protocol"
8 bytes  → reserved (all zeros fine)
20 bytes → info_hash
20 bytes → peer_id
```

From peer - same as above (peer_id of peer obv). 
Make sure that it says "BitTorrent protocol" and the info_hash match

After the handshake all messages between peers follow
```
[length 4_BYTE][message_id 1_BYTE][payload ?_BYTE]

length - length of message_id + payload
message_id - check #common message ids
payload - 
```

Example: `00 00 00 01 02`. length = `00 00 00 01` and message_id = `02`, payload = none

`00 00 00 05 04 00 00 00 0A`. length = `00 00 00 05`, message_id = `04`, payload = `00 00 00 0A`

**Keep Alive** `00 00 00 00`, just to notify that peer is not AFK


```go
// 1. read 4 bytes
length := readUint32()

if length == 0 {
    // keep-alive
    continue
}

// 2. read 1 byte (message id)
msgID := readByte()

// 3. read (length - 1) bytes payload
payload := readBytes(length - 1)
```

#### Common message_ids
| ID | Meaning        |
| -- | -------------- |
| 0  | choke          |
| 1  | unchoke        |
| 2  | interested     |
| 3  | not interested |
| 4  | have           |
| 5  | bitfield       |
| 6  | request        |
| 7  | piece          |

### Bitfield

keep track of the pieces the client has as well as what the peer has. 
So if piece_count = 1000, bits in bitfield = 1000
but bits are packed into bytes. thus bitfield length = `ceil(number_of_pieces / 8)`. The remaining bits stay 0.

Bits ordered, most significant bit last, thus the last bit is first...

Piece `i` would map to -> `byte pos = i // 8` -> `bit pos in that byte = 7 - (i % 8)`


### Single file torrents vs Multi file torrents
Figirl is a multi-file torrent, saves the torrent spanning multiple files in a dir.

If the `info` dict in the torrent file contains a `length` attribute -> its a single file torrent and the `length` is the size of the file. Otherwise if it container `files` array, the elements represent each file name and their size. See below for an example

Example of a multi-file torrent. Add up all the length to get the final length
```
 "files": [
         {
            "length": 278195024,
            "path": [
               "fg-01.bin"
            ]
         },
         {
            "length": 5178402,
            "path": [
               "fg-02.bin"
            ]
         },
```

#### Working with multi-file torrents
Treat all the files as a single file, so say the length was 6, 17, 3
total cumulative file len - 26

In that length, you find the offset for each piece. Since pieces can contain parts of 1 file and the next, eg. `A 4MB piece might contain the last 1MB of file A and the first 3MB of file B. You need to split the write across both files.`

**Startup workflow**

curr_file_len = 6

------
-----------------
---
