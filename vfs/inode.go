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

const (
	InodeFileType      = 0
	InodeDirectoryType = 1
	InodeRootInodeType = 2
)

type Inode struct {
	Type              byte
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

func (i Inode) ReadData(volume ReadWriteVolume, sb Superblock, offset VolumePtr, data []byte) (VolumePtr, error) {
	clusterPtrOffset := ClusterPtr(offset / VolumePtr(sb.ClusterSize))
	offsetInCluster := offset % VolumePtr(sb.ClusterSize)

	dataOffset := VolumePtr(0)
	for {
		clusterPtr, err := i.ResolveDataClusterAddress(volume, sb, clusterPtrOffset)
		if err != nil {
			return dataOffset, err
		}

		clusterData := make([]byte, VolumePtr(sb.ClusterSize)-offsetInCluster)
		err = volume.ReadBytes(ClusterPtrToVolumePtr(sb, clusterPtr)+offsetInCluster, clusterData)
		if err != nil {
			return dataOffset, err
		}

		copy(data[dataOffset:], clusterData)
		clusterPtrOffset++
		offsetInCluster = 0
		dataOffset += VolumePtr(len(clusterData))

		if dataOffset >= VolumePtr(len(data)) {
			break
		}
	}

	return dataOffset, nil
}

func (i Inode) ResolveDataClusterAddress(volume ReadWriteVolume, sb Superblock, index ClusterPtr) (ClusterPtr, error) {
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

	ptrsPerCluster := ClusterPtr(getPtrsPerCluster(sb))
	if index >= InodeDirectCount && index < InodeDirectCount+ptrsPerCluster {
		// Resolve indirect1
		indexInIndirect1 := index - InodeDirectCount

		data := make([]byte, sb.ClusterSize)
		err := volume.ReadBytes(ClusterPtrToVolumePtr(sb, i.Indirect1), data)
		if err != nil {
			return 0, err
		}
		dataClusterPtrs := GetClusterPtrsFromBinary(data)
		return dataClusterPtrs[indexInIndirect1], nil
	} else {
		// Resolve indirect2
		indexInIndirect2 := index - (InodeDirectCount + ptrsPerCluster)

		doublePtrData := make([]byte, sb.ClusterSize)
		err := volume.ReadBytes(ClusterPtrToVolumePtr(sb, i.Indirect2), doublePtrData)
		if err != nil {
			return 0, err
		}
		singleClusterPtrs := GetClusterPtrsFromBinary(doublePtrData)

		singlePtrDataIndex := indexInIndirect2 / ptrsPerCluster
		singlePtrData := make([]byte, sb.ClusterSize)
		err = volume.ReadBytes(ClusterPtrToVolumePtr(sb, singleClusterPtrs[singlePtrDataIndex]), singlePtrData)
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

type MutableInode struct {
	Inode    *Inode
	InodePtr InodePtr
}

func LoadMutableInode(volume ReadWriteVolume, sb Superblock, inodePtr InodePtr) (MutableInode, error) {
	inode := Inode{}
	err := volume.ReadStruct(InodePtrToVolumePtr(sb, inodePtr), &inode)
	if err != nil {
		return MutableInode{}, err
	}

	return MutableInode{
		Inode:    &inode,
		InodePtr: inodePtr,
	}, nil
}

func (mi MutableInode) AppendData(volume ReadWriteVolume, sb Superblock, data []byte) (n VolumePtr, err error) {
	clusterIndex := ClusterPtr(mi.Inode.Size / VolumePtr(sb.ClusterSize))
	indexInCluster := mi.Inode.Size % VolumePtr(sb.ClusterSize)
	sizeBeforeWrite := mi.Inode.Size

	remainingDataLength := VolumePtr(len(data))
	writableSize := VolumePtr(math.Min(float64(remainingDataLength), float64(VolumePtr(sb.ClusterSize)-indexInCluster)))
	startIndex := VolumePtr(0)
	for {
		dataToWrite := make([]byte, writableSize)
		clusterPtr, err := mi.Inode.ResolveDataClusterAddress(volume, sb, clusterIndex)
		if err != nil {
			switch err.(type) {
			case ClusterIndexOutOfRange:
				// Reallocate
				// TODO: sb.ClusterSize is only for testing purposes
				_, err = Allocate(mi, volume, sb, VolumePtr(sb.ClusterSize))
				if err != nil {
					return 0, err
				}
				clusterPtr, err = mi.Inode.ResolveDataClusterAddress(volume, sb, clusterIndex)
				if err != nil {
					return 0, err
				}
			default:
				return 0, err
			}
		}
		copy(dataToWrite, data[startIndex:startIndex+writableSize])
		err = volume.WriteStruct(ClusterPtrToVolumePtr(sb, clusterPtr)+indexInCluster, dataToWrite)
		if err != nil {
			return 0, err
		}

		indexInCluster = 0
		startIndex += writableSize
		mi.Inode.Size += writableSize
		remainingDataLength -= writableSize
		writableSize = VolumePtr(math.Min(float64(remainingDataLength), float64(VolumePtr(sb.ClusterSize))))
		clusterIndex++

		if remainingDataLength <= 0 {
			err = mi.Save(volume, sb)
			if err != nil {
				return mi.Inode.Size - sizeBeforeWrite, err
			}

			return mi.Inode.Size - sizeBeforeWrite, nil
		}
	}
}

func (mi MutableInode) Save(volume ReadWriteVolume, sb Superblock) error {
	return volume.WriteStruct(InodePtrToVolumePtr(sb, mi.InodePtr), mi.Inode)
}
