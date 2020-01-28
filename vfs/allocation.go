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

const Unused = ClusterPtr(-1)

type NoFreeInodeAvailableError struct{}

func (n NoFreeInodeAvailableError) Error() string {
	return "no free inode is available"
}

type NoFreeClusterAvailableError struct{}

func (n NoFreeClusterAvailableError) Error() string {
	return "no free cluster is available"
}

func Allocate(mutableInode MutableInode, volume ReadWriteVolume, sb Superblock, size VolumePtr) (VolumePtr, error) {
	// TODO: Maybe should return VolumeObject because caller doesn't know address of the mutableInode
	// TODO: Do we have enough clusters and space?

	allocatedSize := VolumePtr(0)

	// Allocate direct blocks
	allocatedSizeDirect, err := AllocateDirect(mutableInode.Inode, volume, sb, size)
	if err != nil {
		return 0, err
	}
	size -= allocatedSizeDirect
	allocatedSize += allocatedSizeDirect

	if size > 0 {
		// Allocate indirect1
		allocatedSizeIndirect1, err := AllocateIndirect1(mutableInode.Inode, volume, sb, size)
		if err != nil {
			return 0, err
		}
		size -= allocatedSizeIndirect1
		allocatedSize += allocatedSizeIndirect1

		if size > 0 {
			// Allocate indirect2
			allocatedSizeIndirect2, err := AllocateIndirect2(mutableInode.Inode, volume, sb, size)
			if err != nil {
				return 0, err
			}
			allocatedSize += allocatedSizeIndirect2
		}
	}

	// Save modified inode
	err = volume.WriteStruct(InodePtrToVolumePtr(sb, mutableInode.InodePtr), mutableInode.Inode)
	if err != nil {
		return allocatedSize, err
	}

	return allocatedSize, nil
}

func AllocateDirect(inode *Inode, volume ReadWriteVolume, sb Superblock, size VolumePtr) (VolumePtr, error) {
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

	directPtrs = directPtrs[int(math.Min(float64(inode.AllocatedClusters), InodeDirectCount)):]

	allocatedSize := VolumePtr(0)
	if size > VolumePtr(len(directPtrs)*int(sb.ClusterSize)) {
		size = VolumePtr(len(directPtrs) * int(sb.ClusterSize))
	}
	neededClusters := NeededClusters(sb, size)
	clusterObjects, err := FindFreeClusters(volume, sb, neededClusters, true)
	if err != nil {
		return 0, err
	}

	// Find clusters for direct pointers
	for i := 0; i < len(clusterObjects); i++ {
		clusterPtr := VolumePtrToClusterPtr(sb, clusterObjects[i].VolumePtr)
		*(directPtrs[i]) = clusterPtr
		allocatedSize += VolumePtr(sb.ClusterSize)
		inode.AllocatedClusters++
	}

	return allocatedSize, nil
}

func AllocateIndirect1(inode *Inode, volume ReadWriteVolume, sb Superblock, size VolumePtr) (VolumePtr, error) {
	//volume = NewCachedVolume(volume)
	//defer func() {
	//	_ = volume.(CachedVolume).Flush()
	//}()

	if inode.Indirect1 == Unused {
		// Allocate single pointer table
		singlePtrTableObj, err := FindFreeClusters(volume, sb, 1, true)
		if err != nil {
			return 0, err
		}

		inode.Indirect1 = VolumePtrToClusterPtr(sb, singlePtrTableObj[0].VolumePtr)
	}

	if VolumePtr(inode.AllocatedClusters) >= InodeDirectCount+GetPtrsPerCluster(sb) {
		return 0, nil
	}

	singlePtrTableOffset := AllocatedDataClustersInIndirect1(*inode)
	neededDataClusters := ClusterPtr(math.Min(
		float64(NeededClusters(sb, size)),
		float64(GetPtrsPerCluster(sb)-VolumePtr(singlePtrTableOffset)),
	))

	dataClusterObjects, err := FindFreeClusters(volume, sb, neededDataClusters, true)

	// Convert volume ptrs to cluster ptrs
	singlePtrs := make([]ClusterPtr, neededDataClusters)
	for i := 0; i < len(dataClusterObjects); i++ {
		singlePtrs[i] = VolumePtrToClusterPtr(sb, dataClusterObjects[i].VolumePtr)
		inode.AllocatedClusters++
	}

	err = volume.WriteStruct(
		ClusterPtrToVolumePtr(sb, inode.Indirect1)+(VolumePtr(singlePtrTableOffset)*VolumePtr(unsafe.Sizeof(ClusterPtr(0)))),
		singlePtrs,
	)

	if err != nil {
		return 0, nil
	}

	return VolumePtr(len(singlePtrs)) * VolumePtr(sb.ClusterSize), nil
}

func AllocatedDataClustersInIndirect1(inode Inode) ClusterPtr {
	return inode.AllocatedClusters - InodeDirectCount
}

func AllocateIndirect2(inode *Inode, volume ReadWriteVolume, sb Superblock, size VolumePtr) (VolumePtr, error) {
	//volume = NewCachedVolume(volume)
	//defer func() {
	//	_ = volume.(CachedVolume).Flush()
	//}()

	if inode.Indirect2 == Unused {
		// Allocate double pointer table
		doublePtrTableObj, err := FindFreeClusters(volume, sb, 1, true)
		if err != nil {
			return 0, err
		}

		inode.Indirect2 = VolumePtrToClusterPtr(sb, doublePtrTableObj[0].VolumePtr)
	}

	// Count offsets for double and single pointer tables
	doublePtrTableOffset := AllocatedSinglePtrTablesInIndirect2(*inode, sb)
	singlePtrTableOffset := VolumePtr(AllocatedDataClustersInIndirect2(*inode, sb)) % GetPtrsPerCluster(sb)
	var freePtrsInLastSinglePtrTable VolumePtr
	if singlePtrTableOffset == 0 {
		freePtrsInLastSinglePtrTable = 0
	} else {
		freePtrsInLastSinglePtrTable = GetPtrsPerCluster(sb) - singlePtrTableOffset // Count needed clusters
	}

	neededDataClusters := NeededClusters(sb, size)
	neededNewSinglePtrTables := ClusterPtr(math.Ceil(
		float64(VolumePtr(neededDataClusters)-freePtrsInLastSinglePtrTable) / float64(GetPtrsPerCluster(sb)),
	))

	// Allocate new data clusters
	dataClusterObjects, err := FindFreeClusters(volume, sb, neededDataClusters, true)
	if err != nil {
		return 0, nil
	}

	// Allocate new single pointer clusters
	singlePtrClusterObjects, err := FindFreeClusters(volume, sb, neededNewSinglePtrTables, true)
	if err != nil {
		return 0, nil
	}

	// Add new single pointer tables to the double pointer table
	doublePtrs := make([]ClusterPtr, neededNewSinglePtrTables)
	for i := 0; i < len(singlePtrClusterObjects); i++ {
		doublePtrs[i] = VolumePtrToClusterPtr(sb, singlePtrClusterObjects[i].VolumePtr)
	}
	err = volume.WriteStruct(
		ClusterPtrToVolumePtr(sb, inode.Indirect2)+(VolumePtr(doublePtrTableOffset)*VolumePtr(unsafe.Sizeof(ClusterPtr(0)))),
		doublePtrs,
	)
	if err != nil {
		return 0, err
	}

	if freePtrsInLastSinglePtrTable != 0 {
		// Get pointer of last *old* single pointer table
		lastOldSinglePtrTablePtrByte := make([]byte, unsafe.Sizeof(ClusterPtr(0)))
		err = volume.ReadBytes(
			ClusterPtrToVolumePtr(sb, inode.Indirect2)+(VolumePtr(doublePtrTableOffset-1)*VolumePtr(unsafe.Sizeof(ClusterPtr(0)))),
			lastOldSinglePtrTablePtrByte,
		)
		if err != nil {
			return 0, err
		}
		// Convert byte to ClusterPtr
		lastOldSinglePtrTablePtr := ConvertByteToClusterPtr(lastOldSinglePtrTablePtrByte)

		// Prepend last *old* single pointer table before *new* single pointer tables
		singlePtrClusterObjects = append(
			[]VolumeObject{NewVolumeObject(ClusterPtrToVolumePtr(sb, lastOldSinglePtrTablePtr), volume, nil)},
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
			singlePtrsLen = GetPtrsPerCluster(sb)
		}

		// Write pointers to data clusters to single pointer table
		singlePtrs := make([]ClusterPtr, singlePtrsLen)
		for j := 0; j < len(singlePtrs) && dataClusterPtrIterator < len(dataClusterObjects); j++ {
			singlePtrs[j] = VolumePtrToClusterPtr(
				sb,
				dataClusterObjects[dataClusterPtrIterator].VolumePtr,
			)
			inode.AllocatedClusters++
			allocatedSize += VolumePtr(sb.ClusterSize)
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

func AllocatedSinglePtrTablesInIndirect2(inode Inode, sb Superblock) ClusterPtr {
	return ClusterPtr(math.Ceil(
		float64(inode.AllocatedClusters-InodeDirectCount-ClusterPtr(GetPtrsPerCluster(sb))) / float64(GetPtrsPerCluster(sb)),
	))
}

func AllocatedDataClustersInIndirect2(inode Inode, sb Superblock) ClusterPtr {
	return ClusterPtr(
		float64(inode.AllocatedClusters - InodeDirectCount - ClusterPtr(GetPtrsPerCluster(sb))),
	)
}

func ShrinkDirect(inode *Inode, volume ReadWriteVolume, sb Superblock, size VolumePtr) (VolumePtr, error) {
	clustersToBeUnallocated := size / VolumePtr(sb.ClusterSize)

	directPtrs := []*ClusterPtr{
		&inode.Direct5,
		&inode.Direct4,
		&inode.Direct3,
		&inode.Direct2,
		&inode.Direct1,
	}

	directPtrs = directPtrs[int(math.Min(float64(clustersToBeUnallocated), InodeDirectCount)):]

	deallocateSize := VolumePtr(0)
	if size > VolumePtr(len(directPtrs)*int(sb.ClusterSize)) {
		size = VolumePtr(len(directPtrs) * int(sb.ClusterSize))
	}

	// Find clusters for direct pointers
	for i := 0; i < len(directPtrs); i++ {
		err := FreeCluster(volume, sb, *directPtrs[i])
		if err != nil {
			return deallocateSize, err
		}

		*directPtrs[i] = Unused
		deallocateSize += VolumePtr(sb.ClusterSize)
		inode.AllocatedClusters--
	}

	return deallocateSize, nil
}

func ShrinkIndirect1(inode *Inode, volume ReadWriteVolume, sb Superblock, size VolumePtr) (VolumePtr, error) {
	clustersToBeUnallocated := size / VolumePtr(sb.ClusterSize)
	deallocateSize := VolumePtr(0)

	if size > GetPtrsPerCluster(sb)*VolumePtr(sb.ClusterSize) {
		size = GetPtrsPerCluster(sb) * VolumePtr(sb.ClusterSize)
	}

	ptrsInSinglePtrTable := ClusterPtr(math.Max(float64(inode.AllocatedClusters-InodeDirectCount), 0))
	singlePtrs := make([]ClusterPtr, ptrsInSinglePtrTable)
	err := volume.ReadStruct(ClusterPtrToVolumePtr(sb, inode.Indirect1), singlePtrs)
	if err != nil {
		return deallocateSize, err
	}

	clustersToBeUnallocated = VolumePtr(math.Min(float64(clustersToBeUnallocated), float64(len(singlePtrs))))

	singlePtrs = singlePtrs[VolumePtr(ptrsInSinglePtrTable)-clustersToBeUnallocated:]

	// Find clusters for direct pointers
	for i := len(singlePtrs); i >= 0; i-- {
		err = FreeCluster(volume, sb, singlePtrs[i])
		if err != nil {
			return deallocateSize, err
		}

		deallocateSize += VolumePtr(sb.ClusterSize)
		inode.AllocatedClusters--
	}

	if inode.AllocatedClusters <= InodeDirectCount {
		// Unallocate single pointer table
		err = FreeCluster(volume, sb, inode.Indirect1)
		if err != nil {
			return deallocateSize, err
		}

		inode.Indirect1 = Unused
	}

	return deallocateSize, nil
}

func GetPtrsPerCluster(sb Superblock) VolumePtr {
	var cp ClusterPtr
	clusterPtrSize := int(unsafe.Sizeof(cp))
	ptrsPerCluster := VolumePtr(sb.ClusterSize) / VolumePtr(clusterPtrSize)

	return ptrsPerCluster
}

func FindFreeInode(volume ReadWriteVolume, sb Superblock, occupy bool) (VolumeObject, error) {
	for inodePtr := InodePtr(0); true; inodePtr++ {
		isFree, err := IsInodeFree(volume, sb, inodePtr)
		if err != nil {
			return VolumeObject{}, err
		}

		if isFree {
			if occupy {
				err = OccupyInode(volume, sb, inodePtr)
				if err != nil {
					return VolumeObject{}, err
				}
			}

			inode := NewInode()

			return NewVolumeObject(
				InodePtrToVolumePtr(sb, inodePtr),
				volume,
				inode,
			), nil
		}
	}

	return VolumeObject{}, NoFreeInodeAvailableError{}
}

func IsInodeFree(volume ReadWriteVolume, sb Superblock, ptr InodePtr) (bool, error) {
	bytePtr := sb.InodeBitmapStartAddress + VolumePtr(ptr/8)

	if bytePtr >= sb.InodesStartAddress {
		return false, OutOfRange{bytePtr, sb.InodesStartAddress - 1}
	}

	data, err := volume.ReadByte(bytePtr)
	if err != nil {
		return false, err
	}

	return GetBitInByte(data, int8(ptr%8)) == Free, nil
}

func FreeAllInodes(volume ReadWriteVolume, sb Superblock) error {
Loop:
	for inodePtr := InodePtr(0); true; inodePtr++ {
		err := FreeInode(volume, sb, inodePtr)

		if err != nil {
			switch err.(type) {
			case OutOfRange:
				break Loop
			}
		}
	}

	return nil
}

func setValueInInodeBitmap(volume ReadWriteVolume, sb Superblock, ptr InodePtr, value byte) error {
	bytePtr := sb.InodeBitmapStartAddress + VolumePtr(ptr/8)

	if bytePtr >= sb.InodesStartAddress {
		return OutOfRange{bytePtr, sb.InodesStartAddress - 1}
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

func OccupyInode(volume ReadWriteVolume, sb Superblock, ptr InodePtr) error {
	return setValueInInodeBitmap(volume, sb, ptr, Occupied)
}

func FreeInode(volume ReadWriteVolume, sb Superblock, ptr InodePtr) error {
	return setValueInInodeBitmap(volume, sb, ptr, Free)
}

func NeededClusters(sb Superblock, size VolumePtr) ClusterPtr {
	return ClusterPtr(math.Ceil(float64(size) / float64(sb.ClusterSize)))
}

func FindFreeClusters(volume ReadWriteVolume, sb Superblock, count ClusterPtr, occupy bool) ([]VolumeObject, error) {
	clusterObjects := make([]VolumeObject, 0)

	volumeOffset := VolumePtr(0)
	clusterBitmap := make([]byte, 512)
	for {
		n, err := LoadClusterChunk(volume, sb, volumeOffset, clusterBitmap)
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
					err = OccupyCluster(volume, sb, clusterPtr)
					if err != nil {
						return nil, err
					}
				}

				clusterObjects = append(
					clusterObjects,
					NewVolumeObject(ClusterPtrToVolumePtr(sb, clusterPtr), volume, nil),
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

func LoadClusterChunk(volume ReadWriteVolume, sb Superblock, offset VolumePtr, data []byte) (VolumePtr, error) {
	volumePtr := sb.ClusterBitmapStartAddress + offset

	if volumePtr >= sb.InodeBitmapStartAddress {
		return 0, errors.New(fmt.Sprintf("address can't be equal or greater than %d", sb.InodeBitmapStartAddress))
	}

	err := volume.ReadBytes(volumePtr, data)
	if err != nil {
		return 0, err
	}

	var n VolumePtr
	if sb.ClusterBitmapStartAddress+offset+VolumePtr(len(data)) >= sb.InodeBitmapStartAddress {
		n = sb.InodeBitmapStartAddress - (sb.ClusterBitmapStartAddress + offset)
	} else {
		n = VolumePtr(len(data))
	}

	return n, nil
}

func IsClusterFree(volume ReadWriteVolume, sb Superblock, ptr ClusterPtr) (bool, error) {
	bytePtr := sb.ClusterBitmapStartAddress + VolumePtr(ptr/8)

	if bytePtr >= sb.InodesStartAddress {
		return false, OutOfRange{bytePtr, sb.InodesStartAddress - 1}
	}

	data, err := volume.ReadByte(bytePtr)
	if err != nil {
		return false, err
	}

	return GetBitInByte(data, int8(ptr%8)) == Free, nil
}

func FreeAllClusters(volume ReadWriteVolume, sb Superblock) error {
Loop:
	for inodePtr := ClusterPtr(0); true; inodePtr++ {
		err := FreeCluster(volume, sb, inodePtr)

		if err != nil {
			switch err.(type) {
			case OutOfRange:
				break Loop
			}
		}
	}

	return nil
}

func setValueInClusterBitmap(volume ReadWriteVolume, sb Superblock, ptr ClusterPtr, value byte) error {
	bytePtr := sb.ClusterBitmapStartAddress + VolumePtr(ptr/8)

	if bytePtr >= sb.InodeBitmapStartAddress {
		return OutOfRange{bytePtr, sb.InodeBitmapStartAddress - 1}
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

func OccupyCluster(volume ReadWriteVolume, sb Superblock, ptr ClusterPtr) error {
	return setValueInClusterBitmap(volume, sb, ptr, Occupied)
}

func OccupyClusters(volume ReadWriteVolume, sb Superblock, ptrs []ClusterPtr) error {
	for _, ptr := range ptrs {
		err := OccupyCluster(volume, sb, ptr)
		if err != nil {
			return err
		}
	}

	return nil
}

func FreeCluster(volume ReadWriteVolume, sb Superblock, ptr ClusterPtr) error {
	return setValueInClusterBitmap(volume, sb, ptr, Free)
}
