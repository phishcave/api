package main

import (
	"bytes"
	"testing"
)

func showBin(b uint64) string {
	buf := bytes.Buffer{}
	for i := uint(8); i > 0; i-- {
		buf.WriteByte(byte(((b & (1 << (i - 1))) >> (i - 1)) + '0'))
	}
	return buf.String()
}

func TestChunkList_Set(t *testing.T) {
	cl := make(ChunkList, 2)

	cl.Set(0)
	cl.Set(1)
	cl.Set(64)
	cl.Set(65)

	if cl[0] != 0x3 {
		t.Error("First byte was wrong:", showBin(cl[0]))
	}

	if cl[1] != 0x3 {
		t.Error("Second byte was wrong:", showBin(cl[1]))
	}
}

func TestChunkList_Get(t *testing.T) {
	cl := make(ChunkList, 2)

	cl[0] = 0x3
	cl[1] = 0x3

	if !cl.Has(0) {
		t.Error("Should have")
	}
	if !cl.Has(1) {
		t.Error("Should have")
	}
	if cl.Has(2) {
		t.Error("Should not have")
	}

	if !cl.Has(64) {
		t.Error("Should have")
	}
	if !cl.Has(65) {
		t.Error("Should have")
	}
	if cl.Has(66) {
		t.Error("Should not have")
	}
}

func TestChunkList_IsComplete(t *testing.T) {
	cl := make(ChunkList, 2)

	cl[0] = maxUint64
	cl[1] = 0x3

	if !cl.IsComplete(65) {
		t.Error("Should be complete")
	}
	if cl.IsComplete(66) {
		t.Error("Should not be complete")
	}

	cl[0] &= ^uint64(1)
	if !cl.IsComplete(65) {
		t.Error("Should not be complete")
	}
}

func TestChunkList_ToArray(t *testing.T) {
	cl := make(ChunkList, 2)

	cl[0] = 0x3
	cl[1] = 0x3

	arr := cl.ToArray()
	should := []int{0, 1, 64, 65}

	if len(arr) != len(should) {
		t.Error("Lengths are different:", len(arr))
	}
	for i, sh := range should {
		if sh != arr[i] {
			t.Errorf("%d) Want: %d, Got: %d", i, sh, arr[i])
		}
	}
}
