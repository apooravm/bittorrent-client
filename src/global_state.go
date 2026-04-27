package main

import (
	"sync"
)

type GlobalState struct {
	Bitfield []byte
	mu       sync.RWMutex
}

func (gs *GlobalState) InitBitfield(piece_count int) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// sz := (piece_count + 7) / 8
	gs.Bitfield = make([]byte, piece_count)
}

func (gs *GlobalState) GetBitfield() []byte {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	out := make([]byte, len(gs.Bitfield))
	copy(out, gs.Bitfield)
	return out
}

func (gs *GlobalState) GetPackedBitfield() []byte {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	out := make([]byte, 0)

	for i := 0; i+8 <= len(gs.Bitfield); i += 8 {
		bits := gs.Bitfield[i : i+8]

		var res byte
		for _, b := range bits {
			res = (res << 1) | b
		}

		out = append(out, res)
	}

	return out
}

// needs refactoring
func (gs *GlobalState) SetBitAvailable(idx int) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.Bitfield[idx] = 1

	// b, m := bitPos(idx)
	// gs.Bitfield[b] |= m
}

// needs refactoring
func (gs *GlobalState) CheckBitAvailability(idx int) bool {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	if idx < 0 || idx >= len(gs.Bitfield)*8 {
		return false
	}

	if gs.Bitfield[idx] == 0 {
		return false

	} else {
		return true
	}

	// b, m := bitPos(idx)
	// return gs.Bitfield[b]&m != 0
}

func bitPos(idx int) (byteIdx int, mask byte) {
	byteIdx = idx / 8
	mask = 1 << (7 - idx%8)
	return
}
