package vfs

import (
	"errors"
	"math"
)

type Bitmap []byte

func NewBitmap(size VolumePtr) Bitmap {
	neededMemory := NeededMemoryForBitmap(size)
	return make(Bitmap, neededMemory)
}

func NeededMemoryForBitmap(size VolumePtr) VolumePtr {
	return VolumePtr(math.Ceil(float64(size) / 8))
}

func (b Bitmap) SetBit(position VolumePtr, value byte) error {
	if value != 0 && value != 1 {
		return errors.New("value can be only 0 or 1")
	}

	posInSlice := position / 8

	if posInSlice >= VolumePtr(len(b)) {
		return OutOfRange{posInSlice, VolumePtr(len(b) - 1)}
	}

	posInByte := position % 8
	b[posInSlice] = SetBitInByte(b[posInSlice], int8(posInByte), value)

	return nil
}

func (b Bitmap) GetBit(position VolumePtr) (byte, error) {
	posInSlice := position / 8
	posInByte := position % 8

	if posInSlice >= VolumePtr(len(b)) {
		return 0, OutOfRange{posInSlice, VolumePtr(len(b) - 1)}
	}

	return GetBitInByte(b[posInSlice], int8(posInByte)), nil
}

func (b Bitmap) Zeros() int {
	zeroCount := 0

	for _, _byte := range b {
		for i := 0; i < 8; i++ {
			if (_byte<<i)&0x80 == 0 {
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
			if (_byte<<i)&0x80 == 0x80 {
				onesCount++
			}
		}
	}

	return onesCount
}

func GetBitInByte(data byte, pos int8) byte {
	return (data & (byte(1) << pos)) >> pos
}

func SetBitInByte(data byte, pos int8, value byte) byte {
	if value == 1 {
		data |= byte(1) << pos
	} else {
		data &= ^(byte(1) << pos)
	}

	return data
}
