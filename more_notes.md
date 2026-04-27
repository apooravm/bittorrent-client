Nice—you’ve reached the part where it turns from parsing into actual BitTorrent protocol work. This is where things get *very real*.

I’ll break it down cleanly so you can implement it step by step.

---

# 🧠 Big picture (what happens next)

You now have:

* ✅ peers (IP + port)
* ✅ info hash
* ✅ piece length, piece hashes

Now you need to:

1. Connect to a peer
2. Do the **handshake**
3. Exchange messages (bitfield, interested, etc.)
4. Request blocks
5. Verify pieces

---

# 🔌 1. Connect to a peer

Just a TCP connection:

```go
conn, err := net.Dial("tcp", "IP:PORT")
```

---

# 🤝 2. Handshake (first thing always)

You must send:

```text
<length=19><"BitTorrent protocol">
<8 reserved bytes>
<info_hash (20 bytes)>
<peer_id (20 bytes)>
```

---

## Structure:

```text
1 byte   → 19
19 bytes → "BitTorrent protocol"
8 bytes  → reserved (all zeros fine)
20 bytes → info_hash
20 bytes → peer_id
```

---

## Example in Go (conceptually)

```go
buf := make([]byte, 68)

buf[0] = 19
copy(buf[1:], "BitTorrent protocol")
copy(buf[28:], infoHash[:])
copy(buf[48:], peerID[:])

conn.Write(buf)
```

---

## Then read handshake back

* Must match `info_hash`
* Extract peer ID

---

# 📩 3. Message loop (after handshake)

Messages look like:

```text
<length prefix (4 bytes)>
<message id (1 byte)>
<payload>
```

---

## Important message types

| ID | Name           |
| -- | -------------- |
| 0  | choke          |
| 1  | unchoke        |
| 2  | interested     |
| 3  | not interested |
| 4  | have           |
| 5  | bitfield       |
| 6  | request        |
| 7  | piece          |

---

# ❤️ 4. Tell peer you’re interested

Send:

```text
length = 1
id = 2
```

```go
conn.Write([]byte{0,0,0,1, 2})
```

---

# 🚦 5. Wait for **unchoke**

You cannot request blocks until:

* peer sends message `id = 1`

---

# 🧩 6. Request blocks

## Structure of a request:

```text
length = 13
id = 6
index (4 bytes)
begin (4 bytes)
length (4 bytes)
```

---

## Example

```go
// request piece 0, offset 0, length 16KB
req := make([]byte, 17)

binary.BigEndian.PutUint32(req[0:4], 13)
req[4] = 6
binary.BigEndian.PutUint32(req[5:9], 0)      // piece index
binary.BigEndian.PutUint32(req[9:13], 0)     // offset
binary.BigEndian.PutUint32(req[13:17], 16384) // block size

conn.Write(req)
```

---

## 🔥 Important

* Block size is typically **16KB (16384 bytes)**
* Pieces are split into multiple blocks

---

# 📦 7. Receiving blocks

You’ll get message:

```text
id = 7 (piece)
payload:
  index (4)
  begin (4)
  block data
```

---

## You must:

* collect blocks
* assemble full piece

---

# 🧪 8. Verify piece (CRITICAL)

Each piece has a SHA1 hash from the torrent file.

After assembling full piece:

```go
hash := sha1.Sum(pieceData)
```

Compare with expected hash:

```go
if bytes.Equal(hash[:], expectedHash) {
    // ✅ valid piece
} else {
    // ❌ discard
}
```

---

# 🧠 Where expected hashes come from

From torrent file:

```text
info → pieces
```

This is:

* a long string
* split into 20-byte chunks

Each chunk = SHA1 for a piece

---

# 🔁 9. Loop

For each piece:

1. Request blocks
2. Assemble
3. Verify
4. Move to next

---

# ⚠️ Real-world complications (you can ignore at first)

* Choking/unchoking cycles
* Multiple peers
* Timeouts
* Endgame mode
* Rare piece selection

---

# 🧱 Minimal working strategy

Do this first:

1. Connect to **one peer**
2. Handshake
3. Send interested
4. Wait unchoke
5. Download **just piece 0**
6. Verify it

That alone proves everything works.

---

# 🔥 Debug tips

* Print every message ID you receive
* If nothing happens → you’re probably choked
* If hash fails → block assembly is wrong

---

# 🚀 If you want next step

I can help you:

* build a minimal message loop
* parse bitfield (so you know what pieces peer has)
* or implement piece/block scheduler cleanly

You’re now past parsing—this is basically writing a real BitTorrent client now.

