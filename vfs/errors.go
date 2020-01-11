package vfs

import "fmt"

type OutOfRange struct {
	index    VolumePtr
	maxIndex VolumePtr
}

func (o OutOfRange) Error() string {
	return fmt.Sprintf("idnex out of range [%d], maximal index is [%d]", o.index, o.maxIndex)
}
