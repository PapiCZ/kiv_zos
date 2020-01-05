package vfs

import (
	"errors"
	"fmt"
	"math"
)

type OutOfRange struct {
	index    Vptr
	maxIndex Vptr
}

func (o OutOfRange) Error() string {
	return fmt.Sprintf("idnex out of range [%d], maximal index is [%d]", o.index, o.maxIndex)
}

type Bitmap []byte

func NewBitmap(length Vptr) Bitmap {
	neededMemory := NeededMemoryForBitmap(length)
	return make(Bitmap, neededMemory)
}

func NeededMemoryForBitmap(length Vptr) Vptr {
	return Vptr(math.Ceil(float64(length) / 8))
}

func (b Bitmap) SetBit(position Vptr, value byte) error {
	if value != 0 && value != 1 {
		return errors.New("value can be only 0 or 1")
	}

	posInSlice := position / 8

	if posInSlice >= Vptr(len(b)) {
		return OutOfRange{posInSlice, Vptr(len(b) - 1)}
	}

	posInByte := position % 8

	if value == 1 {
		b[posInSlice] |= byte(1) << posInByte
	} else {
		b[posInSlice] &= ^(byte(1) << posInByte)
	}

	return nil
}

func (b Bitmap) GetBit(position Vptr) (byte, error) {
	posInSlice := position / 8
	posInByte := position % 8

	if posInSlice >= Vptr(len(b)) {
		return 0, OutOfRange{posInSlice, Vptr(len(b) - 1)}
	}

	return b[posInSlice] & (byte(1) << posInByte), nil
}

func (b Bitmap) Zeros() int {
	zeroCount := 0

	for _, _byte := range b {
		for i := 0; i < 8; i++ {
			if (_byte << i) & 0x80 == 0 {
				zeroCount++
			}
		}
	}

	return zeroCount
}

func (b Bitmap) Ones() int {
	onesCount := 0

	for _, _byte := range b {
		for i := 0; i < 8; i++ {
			if (_byte << i) & 0x80 == 0x80 {
				onesCount++
			}
		}
	}

	return onesCount
}
