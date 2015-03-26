package main

const maxUint64 = uint64(1) << 63

type ChunkList []uint64

func NewChunkList(number int) ChunkList {
	return make(ChunkList, number+63/64)
}

func (c ChunkList) Set(number int) {
	c[number/64] |= 1 << uint(number%64)
}

func (c ChunkList) Has(number int) bool {
	return 0 != c[number/64]&(1<<uint(number%64))
}

func (c ChunkList) IsComplete(nChunks int) bool {
	byt := 0
	for nChunks > 64 {
		if c[byt] != maxUint64 {
			return false
		}
		nChunks--
		byt++
	}

	return true
}

func (c ChunkList) ToArray() (arr []int) {
	for i, byt := range c {
		for bit := uint(0); bit < 64; bit += 1 {
			if byt&(1<<bit) != 0 {
				arr = append(arr, i*64+int(bit))
			}
		}
	}

	return arr
}
