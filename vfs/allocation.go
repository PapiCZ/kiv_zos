package vfs

import (
	"errors"
	"fmt"
	"math"
	"unsafe"
)

const (
	Free     = 0
	Occupied = 1
)

const Unused = -1

type NoFreeInodeAvailableError struct{}

func (n NoFreeInodeAvailableError) Error() string {
	return "no free inode is available"
}

type NoFreeClusterAvailableError struct{}

func (n NoFreeClusterAvailableError) Error() string {
	return "no free cluster is available"
}

func Allocate(inode *Inode, volume ReadWriteVolume, superblock Superblock, size VolumePtr) (VolumePtr, error) {
	// TODO: Maybe should return VolumeObject because caller doesn't know address of the inode
	// TODO: Do we have enough clusters and space?

	allocatedSize := VolumePtr(0)

	// Allocate direct blocks
	allocatedSizeDirect, err := AllocateDirect(inode, volume, superblock, size)
	if err != nil {
		return 0, err
	}
	size -= allocatedSizeDirect
	allocatedSize += allocatedSizeDirect

	if size > 0 {
		// Allocate indirect1
		allocatedSizeIndirect1, err := AllocateIndirect1(inode, volume, superblock, size)
		if err != nil {
			return 0, err
		}
		size -= allocatedSizeIndirect1
		allocatedSize += allocatedSizeIndirect1

		if size > 0 {
			// Allocate indirect2
			allocatedSizeIndirect2, err := AllocateIndirect2(inode, volume, superblock, size)
			if err != nil {
				return 0, err
			}
			allocatedSize += allocatedSizeIndirect2
		}
	}

	return allocatedSize, nil
}

func AllocateDirect(inode *Inode, volume ReadWriteVolume, superblock Superblock, size VolumePtr) (VolumePtr, error) {
	//volume = NewCachedVolume(volume)
	//defer func() {
	//	_ = volume.(CachedVolume).Flush()
	//}()

	directPtrs := []*ClusterPtr{
		&inode.Direct1,
		&inode.Direct2,
		&inode.Direct3,
		&inode.Direct4,
		&inode.Direct5,
	}

	directPtrs = directPtrs[int(math.Min(float64(inode.AllocatedClusters), 5)):]

	allocatedSize := VolumePtr(0)
	if size > VolumePtr(len(directPtrs)*int(superblock.ClusterSize)) {
		size = VolumePtr(len(directPtrs) * int(superblock.ClusterSize))
	}
	neededClusters := NeededClusters(superblock, size)
	clusterObjects, err := FindFreeClusters(volume, superblock, neededClusters, true)
	if err != nil {
		return 0, err
	}

	// Find clusters for direct pointers
	for i := 0; i < len(clusterObjects); i++ {
		clusterPtr := VolumePtrToClusterPtr(superblock, clusterObjects[i].VolumePtr)
		*(directPtrs[i]) = clusterPtr
		allocatedSize += VolumePtr(superblock.ClusterSize)
		inode.AllocatedClusters++
	}

	return allocatedSize, nil
}

func AllocateIndirect1(inode *Inode, volume ReadWriteVolume, superblock Superblock, size VolumePtr) (VolumePtr, error) {
	//volume = NewCachedVolume(volume)
	//defer func() {
	//	_ = volume.(CachedVolume).Flush()
	//}()

	if inode.Indirect1 == Unused {
		// Allocate single pointer table
		singlePtrTableObj, err := FindFreeClusters(volume, superblock, 1, true)
		if err != nil {
			return 0, err
		}

		inode.Indirect1 = VolumePtrToClusterPtr(superblock, singlePtrTableObj[0].VolumePtr)
	}

	if VolumePtr(inode.AllocatedClusters) >= InodeDirectCount + GetPtrsPerCluster(superblock) {
		return 0, nil
	}

	singlePtrTableOffset := AllocatedDataClustersInIndirect1(*inode)
	neededDataClusters := ClusterPtr(math.Min(
		float64(NeededClusters(superblock, size)),
		float64(GetPtrsPerCluster(superblock)-VolumePtr(singlePtrTableOffset)),
	))

	dataClusterObjects, err := FindFreeClusters(volume, superblock, neededDataClusters, true)

	// Convert volume ptrs to cluster ptrs
	singlePtrs := make([]ClusterPtr, neededDataClusters)
	for i := 0; i < len(dataClusterObjects); i++ {
		singlePtrs[i] = VolumePtrToClusterPtr(superblock, dataClusterObjects[i].VolumePtr)
		inode.AllocatedClusters++
	}

	err = volume.WriteStruct(
		ClusterPtrToVolumePtr(superblock, inode.Indirect1)+(VolumePtr(singlePtrTableOffset)*VolumePtr(unsafe.Sizeof(ClusterPtr(0)))),
		singlePtrs,
	)

	if err != nil {
		return 0, nil
	}

	return VolumePtr(len(singlePtrs)) * VolumePtr(superblock.ClusterSize), nil
}

func AllocatedDataClustersInIndirect1(inode Inode) ClusterPtr {
	return inode.AllocatedClusters - InodeDirectCount
}

func AllocateIndirect2(inode *Inode, volume ReadWriteVolume, superblock Superblock, size VolumePtr) (VolumePtr, error) {
	//volume = NewCachedVolume(volume)
	//defer func() {
	//	_ = volume.(CachedVolume).Flush()
	//}()

	if inode.Indirect2 == Unused {
		// Allocate double pointer table
		doublePtrTableObj, err := FindFreeClusters(volume, superblock, 1, true)
		if err != nil {
			return 0, err
		}

		inode.Indirect2 = VolumePtrToClusterPtr(superblock, doublePtrTableObj[0].VolumePtr)
	}

	// Count offsets for double and single pointer tables
	doublePtrTableOffset := AllocatedSinglePtrTablesInIndirect2(*inode, superblock)
	singlePtrTableOffset := VolumePtr(AllocatedDataClustersInIndirect2(*inode, superblock)) % GetPtrsPerCluster(superblock)
	var freePtrsInLastSinglePtrTable VolumePtr
	if singlePtrTableOffset == 0 {
		freePtrsInLastSinglePtrTable = 0
	} else {
		freePtrsInLastSinglePtrTable = GetPtrsPerCluster(superblock) - singlePtrTableOffset // Count needed clusters
	}

	neededDataClusters := NeededClusters(superblock, size)
	neededNewSinglePtrTables := ClusterPtr(math.Ceil(
		float64(VolumePtr(neededDataClusters)-freePtrsInLastSinglePtrTable) / float64(GetPtrsPerCluster(superblock)),
	))

	// Allocate new data clusters
	dataClusterObjects, err := FindFreeClusters(volume, superblock, neededDataClusters, true)
	if err != nil {
		return 0, nil
	}

	// Allocate new single pointer clusters
	singlePtrClusterObjects, err := FindFreeClusters(volume, superblock, neededNewSinglePtrTables, true)
	if err != nil {
		return 0, nil
	}

	// Add new single pointer tables to the double pointer table
	doublePtrs := make([]ClusterPtr, neededNewSinglePtrTables)
	for i := 0; i < len(singlePtrClusterObjects); i++ {
		doublePtrs[i] = VolumePtrToClusterPtr(superblock, singlePtrClusterObjects[i].VolumePtr)
	}
	err = volume.WriteStruct(
		ClusterPtrToVolumePtr(superblock, inode.Indirect2)+(VolumePtr(doublePtrTableOffset)*VolumePtr(unsafe.Sizeof(ClusterPtr(0)))),
		doublePtrs,
	)
	if err != nil {
		return 0, err
	}

	if freePtrsInLastSinglePtrTable != 0 {
		// Get pointer of last *old* single pointer table
		lastOldSinglePtrTablePtrByte := make([]byte, unsafe.Sizeof(ClusterPtr(0)))
		err = volume.ReadBytes(
			ClusterPtrToVolumePtr(superblock, inode.Indirect2)+(VolumePtr(doublePtrTableOffset-1)*VolumePtr(unsafe.Sizeof(ClusterPtr(0)))),
			lastOldSinglePtrTablePtrByte,
		)
		if err != nil {
			return 0, err
		}
		// Convert byte to ClusterPtr
		lastOldSinglePtrTablePtr := ConvertByteToClusterPtr(lastOldSinglePtrTablePtrByte)


		// Prepend last *old* single pointer table before *new* single pointer tables
		singlePtrClusterObjects = append(
			[]VolumeObject{NewVolumeObject(ClusterPtrToVolumePtr(superblock, lastOldSinglePtrTablePtr), volume, nil)},
			singlePtrClusterObjects...,
		)
	}

	// Write pointers to data clusters to the single pointer tables
	allocatedSize := VolumePtr(0)
	dataClusterPtrIterator := 0
	for i, singlePtrTableObj := range singlePtrClusterObjects {
		var singlePtrsLen VolumePtr
		if i == 0 && freePtrsInLastSinglePtrTable != 0 {
			// first loop
			singlePtrsLen = freePtrsInLastSinglePtrTable
			// add offset to volume pointer
			singlePtrTableObj.VolumePtr += singlePtrTableOffset * VolumePtr(unsafe.Sizeof(ClusterPtr(0)))
		} else {
			singlePtrsLen = GetPtrsPerCluster(superblock)
		}

		// Write pointers to data clusters to single pointer table
		singlePtrs := make([]ClusterPtr, singlePtrsLen)
		for j := 0; j < len(singlePtrs) && dataClusterPtrIterator < len(dataClusterObjects); j++ {
			singlePtrs[j] = VolumePtrToClusterPtr(
				superblock,
				dataClusterObjects[dataClusterPtrIterator].VolumePtr,
			)
			inode.AllocatedClusters++
			allocatedSize += VolumePtr(superblock.ClusterSize)
			dataClusterPtrIterator++
		}
		singlePtrTableObj.Object = singlePtrs
		err = singlePtrTableObj.Save()
		if err != nil {
			return 0, nil
		}
	}

	return allocatedSize, nil
}

func AllocatedSinglePtrTablesInIndirect2(inode Inode, superblock Superblock) ClusterPtr {
	return ClusterPtr(math.Ceil(
		float64(inode.AllocatedClusters-InodeDirectCount-ClusterPtr(GetPtrsPerCluster(superblock))) / float64(GetPtrsPerCluster(superblock)),
	))
}

func AllocatedDataClustersInIndirect2(inode Inode, superblock Superblock) ClusterPtr {
	return ClusterPtr(
		float64(inode.AllocatedClusters-InodeDirectCount-ClusterPtr(GetPtrsPerCluster(superblock))),
	)
}

func GetPtrsPerCluster(superblock Superblock) VolumePtr {
	var cp ClusterPtr
	clusterPtrSize := int(unsafe.Sizeof(cp))
	ptrsPerCluster := VolumePtr(superblock.ClusterSize) / VolumePtr(clusterPtrSize)

	return ptrsPerCluster
}

func FindFreeInode(volume ReadWriteVolume, superblock Superblock) (VolumeObject, error) {
	for inodePtr := InodePtr(0); true; inodePtr++ {
		isFree, err := IsInodeFree(volume, superblock, inodePtr)
		if err != nil {
			return VolumeObject{}, err
		}

		if isFree {
			inode := NewInode()

			// We don't need real data
			//err := volume.ReadStruct(InodePtrToVolumePtr(superblock, inodePtr), &inode)
			//if err != nil {
			//	return VolumeObject{}, err
			//}

			return NewVolumeObject(
				InodePtrToVolumePtr(superblock, inodePtr),
				volume,
				inode,
			), nil
		}
	}

	return VolumeObject{}, NoFreeInodeAvailableError{}
}

func IsInodeFree(volume ReadWriteVolume, superblock Superblock, ptr InodePtr) (bool, error) {
	bytePtr := superblock.InodeBitmapStartAddress + VolumePtr(ptr/8)

	if bytePtr >= superblock.InodesStartAddress {
		return false, OutOfRange{bytePtr, superblock.InodesStartAddress - 1}
	}

	data, err := volume.ReadByte(bytePtr)
	if err != nil {
		return false, err
	}

	return GetBitInByte(data, int8(ptr%8)) == Free, nil
}

func FreeAllInodes(volume ReadWriteVolume, superblock Superblock) error {
Loop:
	for inodePtr := InodePtr(0); true; inodePtr++ {
		err := FreeInode(volume, superblock, inodePtr)

		if err != nil {
			switch err.(type) {
			case OutOfRange:
				break Loop
			}
		}
	}

	return nil
}

func setValueInInodeBitmap(volume ReadWriteVolume, superblock Superblock, ptr InodePtr, value byte) error {
	bytePtr := superblock.InodeBitmapStartAddress + VolumePtr(ptr/8)

	if bytePtr >= superblock.InodesStartAddress {
		return OutOfRange{bytePtr, superblock.InodesStartAddress - 1}
	}

	data, err := volume.ReadByte(bytePtr)
	if err != nil {
		return err
	}

	data = SetBitInByte(data, int8(ptr%8), value)

	err = volume.WriteByte(bytePtr, data)
	if err != nil {
		return err
	}

	return nil

}

func OccupyInode(volume ReadWriteVolume, superblock Superblock, ptr InodePtr) error {
	return setValueInInodeBitmap(volume, superblock, ptr, Occupied)
}

func FreeInode(volume ReadWriteVolume, superblock Superblock, ptr InodePtr) error {
	return setValueInInodeBitmap(volume, superblock, ptr, Free)
}

func NeededClusters(superblock Superblock, size VolumePtr) ClusterPtr {
	return ClusterPtr(math.Ceil(float64(size) / float64(superblock.ClusterSize)))
}

func FindFreeClusters(volume ReadWriteVolume, superblock Superblock, count ClusterPtr, occupy bool) ([]VolumeObject, error) {
	clusterObjects := make([]VolumeObject, 0)

	volumeOffset := VolumePtr(0)
	clusterBitmap := make([]byte, 512)
	for {
		n, err := LoadClusterChunk(volume, superblock, volumeOffset, clusterBitmap)
		if err != nil {
			return nil, err
		}

		// Find zero bits in byte
		bitmap := Bitmap(clusterBitmap[:n])
		for clusterPtr := ClusterPtr(volumeOffset * 8); clusterPtr < ClusterPtr(volumeOffset*8)+ClusterPtr(n*8); clusterPtr++ {
			value, err := bitmap.GetBit(VolumePtr(clusterPtr) - (volumeOffset * 8))
			if err != nil {
				return nil, err
			}

			if value == Free {
				if occupy {
					err = OccupyCluster(volume, superblock, clusterPtr)
					if err != nil {
						return nil, err
					}
				}

				clusterObjects = append(
					clusterObjects,
					NewVolumeObject(ClusterPtrToVolumePtr(superblock, clusterPtr), volume, nil),
				)
			}

			if ClusterPtr(len(clusterObjects)) == count {
				return clusterObjects, nil
			}
		}

		if n != VolumePtr(len(clusterBitmap)) {
			return nil, errors.New("not enough available cluster")
		}

		volumeOffset += n
	}
}

func LoadClusterChunk(volume ReadWriteVolume, superblock Superblock, offset VolumePtr, data []byte) (VolumePtr, error) {
	volumePtr := superblock.ClusterBitmapStartAddress + offset

	if volumePtr >= superblock.InodeBitmapStartAddress {
		return 0, errors.New(fmt.Sprintf("address can't be equal or greater than %d", superblock.InodeBitmapStartAddress))
	}

	err := volume.ReadBytes(volumePtr, data)
	if err != nil {
		return 0, err
	}

	var n VolumePtr
	if superblock.ClusterBitmapStartAddress+offset+VolumePtr(len(data)) >= superblock.InodeBitmapStartAddress {
		n = superblock.InodeBitmapStartAddress - (superblock.ClusterBitmapStartAddress+offset)
	} else {
		n = VolumePtr(len(data))
	}

	return n, nil
}

func IsClusterFree(volume ReadWriteVolume, superblock Superblock, ptr ClusterPtr) (bool, error) {
	bytePtr := superblock.ClusterBitmapStartAddress + VolumePtr(ptr/8)

	if bytePtr >= superblock.InodesStartAddress {
		return false, OutOfRange{bytePtr, superblock.InodesStartAddress - 1}
	}

	data, err := volume.ReadByte(bytePtr)
	if err != nil {
		return false, err
	}

	return GetBitInByte(data, int8(ptr%8)) == Free, nil
}

func FreeAllClusters(volume ReadWriteVolume, superblock Superblock) error {
Loop:
	for inodePtr := ClusterPtr(0); true; inodePtr++ {
		err := FreeCluster(volume, superblock, inodePtr)

		if err != nil {
			switch err.(type) {
			case OutOfRange:
				break Loop
			}
		}
	}

	return nil
}

func setValueInClusterBitmap(volume ReadWriteVolume, superblock Superblock, ptr ClusterPtr, value byte) error {
	bytePtr := superblock.ClusterBitmapStartAddress + VolumePtr(ptr/8)

	if bytePtr >= superblock.InodeBitmapStartAddress {
		return OutOfRange{bytePtr, superblock.InodeBitmapStartAddress - 1}
	}

	data, err := volume.ReadByte(bytePtr)
	if err != nil {
		return err
	}

	data = SetBitInByte(data, int8(ptr%8), value)

	err = volume.WriteByte(bytePtr, data)
	if err != nil {
		return err
	}

	return nil

}

func OccupyCluster(volume ReadWriteVolume, superblock Superblock, ptr ClusterPtr) error {
	return setValueInClusterBitmap(volume, superblock, ptr, Occupied)
}

func OccupyClusters(volume ReadWriteVolume, superblock Superblock, ptrs []ClusterPtr) error {
	for _, ptr := range ptrs {
		err := OccupyCluster(volume, superblock, ptr)
		if err != nil {
			return err
		}
	}

	return nil
}

func FreeCluster(volume ReadWriteVolume, superblock Superblock, ptr ClusterPtr) error {
	return setValueInClusterBitmap(volume, superblock, ptr, Free)
}
