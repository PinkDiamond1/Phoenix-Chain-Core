package trie

import (
	"bytes"
	"github.com/PhoenixGlobal/Phoenix-Chain-Core/libs/common/hexutil"
	"testing"
)

func TestC(t *testing.T)  {
	t.Log(hexutil.Encode(compactToHex(hexutil.MustDecode("0x30f692b256e24009bcb34d0ee84da73c298afacc0924e01105e2eb0f01a87fe2"))))
}
func TestHexCompact(t *testing.T) {
	tests := []struct{ hex, compact []byte }{
		// empty keys, with and without terminator.
		{hex: []byte{}, compact: []byte{0x00}},
		{hex: []byte{16}, compact: []byte{0x20}},
		// odd length, no terminator
		{hex: []byte{1, 2, 3, 4, 5}, compact: []byte{0x11, 0x23, 0x45}},
		// even length, no terminator
		{hex: []byte{0, 1, 2, 3, 4, 5}, compact: []byte{0x00, 0x01, 0x23, 0x45}},
		// odd length, terminator
		{hex: []byte{15, 1, 12, 11, 8, 16 /*term*/}, compact: []byte{0x3f, 0x1c, 0xb8}},
		// even length, terminator
		{hex: []byte{0, 15, 1, 12, 11, 8, 16 /*term*/}, compact: []byte{0x20, 0x0f, 0x1c, 0xb8}},
	}
	for _, test := range tests {
		if c := hexToCompact(test.hex); !bytes.Equal(c, test.compact) {
			t.Errorf("hexToCompact(%x) -> %x, want %x", test.hex, c, test.compact)
		}
		if h := compactToHex(test.compact); !bytes.Equal(h, test.hex) {
			t.Errorf("compactToHex(%x) -> %x, want %x", test.compact, h, test.hex)
		}
	}
}

func TestHexKeybytes(t *testing.T) {
	tests := []struct{ key, hexIn, hexOut []byte }{
		{key: []byte{}, hexIn: []byte{16}, hexOut: []byte{16}},
		{key: []byte{}, hexIn: []byte{}, hexOut: []byte{16}},
		{
			key:    []byte{0x12, 0x34, 0x56},
			hexIn:  []byte{1, 2, 3, 4, 5, 6, 16},
			hexOut: []byte{1, 2, 3, 4, 5, 6, 16},
		},
		{
			key:    []byte{0x12, 0x34, 0x5},
			hexIn:  []byte{1, 2, 3, 4, 0, 5, 16},
			hexOut: []byte{1, 2, 3, 4, 0, 5, 16},
		},
		{
			key:    []byte{0x12, 0x34, 0x56},
			hexIn:  []byte{1, 2, 3, 4, 5, 6},
			hexOut: []byte{1, 2, 3, 4, 5, 6, 16},
		},
	}
	for _, test := range tests {
		if h := keybytesToHex(test.key); !bytes.Equal(h, test.hexOut) {
			t.Errorf("keybytesToHex(%x) -> %x, want %x", test.key, h, test.hexOut)
		}
		if k := hexToKeybytes(test.hexIn); !bytes.Equal(k, test.key) {
			t.Errorf("hexToKeybytes(%x) -> %x, want %x", test.hexIn, k, test.key)
		}
	}
}

func BenchmarkHexToCompact(b *testing.B) {
	testBytes := []byte{0, 15, 1, 12, 11, 8, 16 /*term*/}
	for i := 0; i < b.N; i++ {
		hexToCompact(testBytes)
	}
}

func BenchmarkCompactToHex(b *testing.B) {
	testBytes := []byte{0, 15, 1, 12, 11, 8, 16 /*term*/}
	for i := 0; i < b.N; i++ {
		compactToHex(testBytes)
	}
}

func BenchmarkKeybytesToHex(b *testing.B) {
	testBytes := []byte{7, 6, 6, 5, 7, 2, 6, 2, 16}
	for i := 0; i < b.N; i++ {
		keybytesToHex(testBytes)
	}
}

func BenchmarkHexToKeybytes(b *testing.B) {
	testBytes := []byte{7, 6, 6, 5, 7, 2, 6, 2, 16}
	for i := 0; i < b.N; i++ {
		hexToKeybytes(testBytes)
	}
}
