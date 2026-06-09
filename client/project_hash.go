package client

import (
	"encoding/binary"
	"math/bits"
	"strconv"
)

const (
	maxProjectDirectoryNameLength = 200
	wyhashSecret0                 = 0xa0761d6478bd642f
	wyhashSecret1                 = 0xe7037ed1a0b428db
	wyhashSecret2                 = 0x8ebc6af09c88c6e3
	wyhashSecret3                 = 0x589965cc75374cc3
)

func projectPathHashSuffix(path string) string {
	return strconv.FormatUint(zigWyhash([]byte(path), 0), 36)
}

// zigWyhash 复现 Zig std.hash.Wyhash.hash(seed, input)，也就是 Bun.hash 的底层规则。
func zigWyhash(input []byte, seed uint64) uint64 {
	state0 := seed ^ wyhashMix(seed^wyhashSecret0, wyhashSecret1)
	state1 := state0
	state2 := state0
	var a uint64
	var b uint64

	if len(input) <= 16 {
		a, b = wyhashSmallKey(input)
	} else {
		index := 0
		if len(input) >= 48 {
			for index+48 < len(input) {
				state0 = wyhashMix(read64(input[index:])^wyhashSecret1, read64(input[index+8:])^state0)
				state1 = wyhashMix(read64(input[index+16:])^wyhashSecret2, read64(input[index+24:])^state1)
				state2 = wyhashMix(read64(input[index+32:])^wyhashSecret3, read64(input[index+40:])^state2)
				index += 48
			}
			state0 ^= state1 ^ state2
		}
		for offset := index; offset+16 < len(input); offset += 16 {
			state0 = wyhashMix(read64(input[offset:])^wyhashSecret1, read64(input[offset+8:])^state0)
		}
		a = read64(input[len(input)-16:])
		b = read64(input[len(input)-8:])
	}

	a ^= wyhashSecret1
	b ^= state0
	low, high := wyhashMum(a, b)
	return wyhashMix(low^wyhashSecret0^uint64(len(input)), high^wyhashSecret1)
}

func wyhashSmallKey(input []byte) (uint64, uint64) {
	if len(input) >= 4 {
		end := len(input) - 4
		quarter := (len(input) >> 3) << 2
		a := read32(input) << 32
		a |= read32(input[quarter:])
		b := read32(input[end:]) << 32
		b |= read32(input[end-quarter:])
		return a, b
	}
	if len(input) > 0 {
		a := uint64(input[0]) << 16
		a |= uint64(input[len(input)>>1]) << 8
		a |= uint64(input[len(input)-1])
		return a, 0
	}
	return 0, 0
}

func wyhashMix(a uint64, b uint64) uint64 {
	low, high := wyhashMum(a, b)
	return low ^ high
}

func wyhashMum(a uint64, b uint64) (uint64, uint64) {
	high, low := bits.Mul64(a, b)
	return low, high
}

func read32(input []byte) uint64 {
	return uint64(binary.LittleEndian.Uint32(input[:4]))
}

func read64(input []byte) uint64 {
	return binary.LittleEndian.Uint64(input[:8])
}
