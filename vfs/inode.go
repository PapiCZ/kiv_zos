package vfs

import (
	"fmt"
	"math"
	"unsafe"
)

const InodeDirectCount = 5

type ClusterIndexOutOfRange struct {
	index ClusterPtr
}

func (c ClusterIndexOutOfRange) Error() string {
	return fmt.Sprintf("index out of range [%d], maximal index is [%d]", c.index, 0)
}

type Inode struct {
	Directory         bool
	Size              VolumePtr
	AllocatedClusters ClusterPtr
	Direct1           ClusterPtr
	Direct2           ClusterPtr
	Direct3           ClusterPtr
	Direct4           ClusterPtr
	Direct5           ClusterPtr
	Indirect1         ClusterPtr
	Indirect2         ClusterPtr
}

func NewInode() Inode {
	return Inode{
		Direct1:   Unused,
		Direct2:   Unused,
		Direct3:   Unused,
		Direct4:   Unused,
		Direct5:   Unused,
		Indirect1: Unused,
		Indirect2: Unused,
	}
}

func (i *Inode) AppendData(volume Volume, superblock Superblock, data []byte) (n VolumePtr, err error) {
	clusterIndex := ClusterPtr(i.Size / VolumePtr(superblock.ClusterSize))
	indexInCluster := i.Size % VolumePtr(superblock.ClusterSize)
	sizeBeforeWrite := i.Size

	remainingDataLength := VolumePtr(len(data))
	writableSize := VolumePtr(math.Min(float64(remainingDataLength), float64(VolumePtr(superblock.ClusterSize)-indexInCluster)))
	startIndex := VolumePtr(0)
	for {
		dataToWrite := make([]byte, writableSize)
		clusterPtr, err := i.ResolveDataClusterAddress(volume, superblock, clusterIndex)
		if err != nil {
			switch err.(type) {
			case ClusterIndexOutOfRange:
				// Reallocate
				// TODO: 4096 is only for testing purposes
				_, err = Allocate(i, volume, superblock, VolumePtr(superblock.ClusterSize))
				if err != nil {
					return 0, err
				}
				clusterPtr, err = i.ResolveDataClusterAddress(volume, superblock, clusterIndex)
				if err != nil {
					return 0, err
				}
			default:
				return 0, err
			}
		}
		copy(dataToWrite, data[startIndex:startIndex+writableSize])
		err = volume.WriteStruct(ClusterPtrToVolumePtr(superblock, clusterPtr)+indexInCluster, dataToWrite)
		if err != nil {
			return 0, err
		}

		indexInCluster = 0
		startIndex += writableSize
		i.Size += writableSize
		remainingDataLength -= writableSize
		writableSize = VolumePtr(math.Min(float64(remainingDataLength), float64(VolumePtr(superblock.ClusterSize))))
		clusterIndex++

		if remainingDataLength <= 0 {
			return i.Size - sizeBeforeWrite, nil
		}
	}
}

func (i Inode) ResolveDataClusterAddress(volume Volume, superblock Superblock, index ClusterPtr) (ClusterPtr, error) {
	// TODO: math.Ceil?
	if index >= i.AllocatedClusters {
		return 0, ClusterIndexOutOfRange{index}
	}

	// Resolve direct
	if index == 0 {
		return i.Direct1, nil
	} else if index == 1 {
		return i.Direct2, nil
	} else if index == 2 {
		return i.Direct3, nil
	} else if index == 3 {
		return i.Direct4, nil
	} else if index == 4 {
		return i.Direct5, nil
	}

	ptrsPerCluster := ClusterPtr(GetPtrsPerCluster(superblock))
	if index >= InodeDirectCount && index < InodeDirectCount+ptrsPerCluster {
		// Resolve indirect1
		indexInIndirect1 := index - InodeDirectCount

		data := make([]byte, superblock.ClusterSize)
		err := volume.ReadBytes(ClusterPtrToVolumePtr(superblock, i.Indirect1), data)
		if err != nil {
			return 0, err
		}
		dataClusterPtrs := GetClusterPtrsFromBinary(data)
		return dataClusterPtrs[indexInIndirect1], nil
	} else {
		// Resolve indirect2
		indexInIndirect2 := index - (InodeDirectCount + ptrsPerCluster)

		doublePtrData := make([]byte, superblock.ClusterSize)
		err := volume.ReadBytes(ClusterPtrToVolumePtr(superblock, i.Indirect2), doublePtrData)
		if err != nil {
			return 0, err
		}
		singleClusterPtrs := GetClusterPtrsFromBinary(doublePtrData)

		singlePtrDataIndex := indexInIndirect2 / ptrsPerCluster
		singlePtrData := make([]byte, superblock.ClusterSize)
		err = volume.ReadBytes(ClusterPtrToVolumePtr(superblock, singleClusterPtrs[singlePtrDataIndex]), singlePtrData)
		if err != nil {
			return 0, err
		}
		dataClusterPtrs := GetClusterPtrsFromBinary(singlePtrData)

		dataPtrIndex := indexInIndirect2 % ptrsPerCluster

		return dataClusterPtrs[dataPtrIndex], nil
	}
}

func GetClusterPtrsFromBinary(p []byte) []ClusterPtr {
	var cp ClusterPtr
	clusterPtrSize := int(unsafe.Sizeof(cp))
	ptrCount := len(p) / clusterPtrSize

	clusterPtrs := make([]ClusterPtr, 0, ptrCount)
	for i := 0; i < ptrCount; i++ {
		clusterBinaryPtr := p[i*clusterPtrSize : (i+1)*clusterPtrSize]

		var clusterPtr ClusterPtr
		for j := 0; j < clusterPtrSize; j++ {
			clusterPtr |= ClusterPtr(clusterBinaryPtr[j]) << ClusterPtr(8*j)
		}

		clusterPtrs = append(clusterPtrs, clusterPtr)
	}

	return clusterPtrs
}
