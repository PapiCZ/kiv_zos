package vfs

import (
	"errors"
	"fmt"
	"math"
)

type OutOfRange struct {
	index    int
	maxIndex int
}

func (o OutOfRange) Error() string {
	return fmt.Sprintf("idnex out of range [%d], maximal index is [%d]", o.index, o.maxIndex)
}

type Bitmap []byte

func NewBitmap(length int32) Bitmap {
	return make(Bitmap, NeededMemoryForBitmap(length))
}

func NeededMemoryForBitmap(length int32) int32 {
	return int32(math.Ceil(float64(length) / 8))
}

func (b Bitmap) SetBit(position int32, value byte) error {
	if value != 0 && value != 1 {
		return errors.New("value can be only 0 or 1")
	}

	posInSlice := position / 8

	if posInSlice >= int32(len(b)) {
		return OutOfRange{int(posInSlice), len(b) - 1}
	}

	posInByte := position % 8

	if value == 1 {
		b[posInSlice] |= byte(1) << posInByte
	} else {
		b[posInSlice] &= ^(byte(1) << posInByte)
	}

	return nil
}

func (b Bitmap) GetBit(position int32) (byte, error) {
	posInSlice := position / 8
	posInByte := position % 8

	if posInSlice >= int32(len(b)) {
		return 0, OutOfRange{int(posInSlice), len(b) - 1}
	}

	return b[posInSlice] & (byte(1) << posInByte), nil
}
