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
	allocatedSizeDirect, err := allocateDirect(mutableInode.Inode, volume, sb, size)
	if err != nil {
		return 0, err
	}
	size -= allocatedSizeDirect
	allocatedSize += allocatedSizeDirect

	if size > 0 {
		// Allocate indirect1
		allocatedSizeIndirect1, err := allocateIndirect1(mutableInode.Inode, volume, sb, size)
		if err != nil {
			return 0, err
		}
		size -= allocatedSizeIndirect1
		allocatedSize += allocatedSizeIndirect1

		if size > 0 {
			// Allocate indirect2
			allocatedSizeIndirect2, err := allocateIndirect2(mutableInode.Inode, volume, sb, size)
			if err != nil {
				return 0, err
			}
			allocatedSize += allocatedSizeIndirect2
		}
	}

	// Save modified inode
	err = mutableInode.Save(volume, sb)
	if err != nil {
		return allocatedSize, err
	}

	return allocatedSize, nil
}

func allocateDirect(inode *Inode, volume ReadWriteVolume, sb Superblock, size VolumePtr) (VolumePtr, error) {
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

func allocateIndirect1(inode *Inode, volume ReadWriteVolume, sb Superblock, size VolumePtr) (VolumePtr, error) {
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

	if VolumePtr(inode.AllocatedClusters) >= InodeDirectCount+getPtrsPerCluster(sb) {
		return 0, nil
	}

	singlePtrTableOffset := allocatedDataClustersInIndirect1(*inode, sb)
	neededDataClusters := ClusterPtr(math.Min(
		float64(NeededClusters(sb, size)),
		float64(getPtrsPerCluster(sb)-VolumePtr(singlePtrTableOffset)),
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

func allocatedDataClustersInIndirect1(inode Inode, sb Superblock) ClusterPtr {
	result := inode.AllocatedClusters - InodeDirectCount

	if result > 0 {
		if VolumePtr(result) > getPtrsPerCluster(sb) {
			return ClusterPtr(getPtrsPerCluster(sb))
		}
		return result
	} else {
		return 0
	}
}

func allocateIndirect2(inode *Inode, volume ReadWriteVolume, sb Superblock, size VolumePtr) (VolumePtr, error) {
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
	doublePtrTableOffset := allocatedSinglePtrTablesInIndirect2(*inode, sb)
	singlePtrTableOffset := VolumePtr(allocatedDataClustersInIndirect2(*inode, sb)) % getPtrsPerCluster(sb)
	var freePtrsInLastSinglePtrTable VolumePtr
	if singlePtrTableOffset == 0 {
		freePtrsInLastSinglePtrTable = 0
	} else {
		freePtrsInLastSinglePtrTable = getPtrsPerCluster(sb) - singlePtrTableOffset // Count needed clusters
	}

	neededDataClusters := NeededClusters(sb, size)
	neededNewSinglePtrTables := ClusterPtr(math.Ceil(
		float64(VolumePtr(neededDataClusters)-freePtrsInLastSinglePtrTable) / float64(getPtrsPerCluster(sb)),
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
			singlePtrsLen = getPtrsPerCluster(sb)
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

func allocatedSinglePtrTablesInIndirect2(inode Inode, sb Superblock) ClusterPtr {
	result := ClusterPtr(math.Ceil(
		float64(inode.AllocatedClusters-InodeDirectCount-ClusterPtr(getPtrsPerCluster(sb))) / float64(getPtrsPerCluster(sb)),
	))

	if result > 0 {
		return result
	} else {
		return 0
	}
}

func allocatedDataClustersInIndirect2(inode Inode, sb Superblock) ClusterPtr {
	result := ClusterPtr(
		float64(inode.AllocatedClusters - InodeDirectCount - ClusterPtr(getPtrsPerCluster(sb))),
	)

	if result > 0 {
		return result
	} else {
		return 0
	}
}

func Shrink(mutableInode MutableInode, volume ReadWriteVolume, sb Superblock, targetSize VolumePtr) (VolumePtr, error) {
	newAllocatedSize, err := shrinkIndirect2(mutableInode.Inode, volume, sb, targetSize)
	if err != nil {
		return newAllocatedSize, err
	}

	if newAllocatedSize >= targetSize {
		newAllocatedSize, err = shrinkIndirect1(mutableInode.Inode, volume, sb, targetSize)
		if err != nil {
			return newAllocatedSize, err
		}

		if newAllocatedSize >= targetSize {
			newAllocatedSize, err = shrinkDirect(mutableInode.Inode, volume, sb, targetSize)
			if err != nil {
				return newAllocatedSize, err
			}
		}
	}

	mutableInode.Inode.Size = 0
	err = mutableInode.Save(volume, sb)
	if err != nil {
		return newAllocatedSize, err
	}

	return newAllocatedSize, nil
}

func shrinkDirect(inode *Inode, volume ReadWriteVolume, sb Superblock, targetSize VolumePtr) (VolumePtr, error) {
	sizeToBeDeallocated := (VolumePtr(inode.AllocatedClusters) * VolumePtr(sb.ClusterSize)) - targetSize
	clustersToBeDeallocated := sizeToBeDeallocated / VolumePtr(sb.ClusterSize)

	directPtrs := []*ClusterPtr{
		&inode.Direct5,
		&inode.Direct4,
		&inode.Direct3,
		&inode.Direct2,
		&inode.Direct1,
	}

	allocatedSize := VolumePtr(inode.AllocatedClusters) * VolumePtr(sb.ClusterSize)
	if sizeToBeDeallocated > VolumePtr(len(directPtrs)*int(sb.ClusterSize)) {
		sizeToBeDeallocated = VolumePtr(len(directPtrs) * int(sb.ClusterSize))
	}

	// Find clusters for direct pointers
	for i := 0; i < len(directPtrs); i++ {
		if *directPtrs[i] == Unused {
			continue
		}

		err := FreeCluster(volume, sb, *directPtrs[i])
		if err != nil {
			return allocatedSize, err
		}

		*directPtrs[i] = Unused
		allocatedSize -= VolumePtr(sb.ClusterSize)
		inode.AllocatedClusters--
		clustersToBeDeallocated--

		if clustersToBeDeallocated == 0 {
			break
		}
	}

	return allocatedSize, nil
}

func shrinkIndirect1(inode *Inode, volume ReadWriteVolume, sb Superblock, targetSize VolumePtr) (VolumePtr, error) {
	sizeToBeDeallocated := (VolumePtr(inode.AllocatedClusters) * VolumePtr(sb.ClusterSize)) - targetSize
	clustersToBeDeallocated := sizeToBeDeallocated / VolumePtr(sb.ClusterSize)
	allocatedSize := VolumePtr(inode.AllocatedClusters) * VolumePtr(sb.ClusterSize)

	if sizeToBeDeallocated > getPtrsPerCluster(sb)*VolumePtr(sb.ClusterSize) {
		sizeToBeDeallocated = getPtrsPerCluster(sb) * VolumePtr(sb.ClusterSize)
	}

	ptrsInSinglePtrTable := allocatedDataClustersInIndirect1(*inode, sb)
	singlePtrs := make([]ClusterPtr, ptrsInSinglePtrTable)
	err := volume.ReadStruct(ClusterPtrToVolumePtr(sb, inode.Indirect1), singlePtrs)
	if err != nil {
		return allocatedSize, err
	}

	clustersToBeDeallocated = VolumePtr(math.Min(float64(clustersToBeDeallocated), float64(len(singlePtrs))))

	singlePtrs = singlePtrs[VolumePtr(ptrsInSinglePtrTable)-clustersToBeDeallocated:]

	// Find clusters for direct pointers
	for i := len(singlePtrs) - 1; i >= 0; i-- {
		err = FreeCluster(volume, sb, singlePtrs[i])
		if err != nil {
			return allocatedSize, err
		}

		allocatedSize -= VolumePtr(sb.ClusterSize)
		inode.AllocatedClusters--
	}

	if allocatedDataClustersInIndirect1(*inode, sb) <= 0  && inode.Indirect1 != Unused  {
		// Deallocate single pointer table
		err = FreeCluster(volume, sb, inode.Indirect1)
		if err != nil {
			return allocatedSize, err
		}

		inode.Indirect1 = Unused
	}

	return allocatedSize, nil
}

func shrinkIndirect2(inode *Inode, volume ReadWriteVolume, sb Superblock, targetSize VolumePtr) (VolumePtr, error) {
	sizeToBeDeallocated := (VolumePtr(inode.AllocatedClusters) * VolumePtr(sb.ClusterSize)) - targetSize
	clustersToBeDeallocated := sizeToBeDeallocated / VolumePtr(sb.ClusterSize)
	allocatedSize := VolumePtr(inode.AllocatedClusters) * VolumePtr(sb.ClusterSize)

	if sizeToBeDeallocated > getPtrsPerCluster(sb)*VolumePtr(sb.ClusterSize)*VolumePtr(sb.ClusterSize) {
		sizeToBeDeallocated = getPtrsPerCluster(sb) * VolumePtr(sb.ClusterSize) * VolumePtr(sb.ClusterSize)
	}

	// Load double pointer table
	ptrsInDoublePtrTable := allocatedSinglePtrTablesInIndirect2(*inode, sb)
	doublePtrs := make([]ClusterPtr, ptrsInDoublePtrTable)
	err := volume.ReadStruct(ClusterPtrToVolumePtr(sb, inode.Indirect2), doublePtrs)
	if err != nil {
		return allocatedSize, err
	}

	// Read double pointer table in reverse order
	stop := false
	for i := len(doublePtrs) - 1; i >= 0; i-- {
		// Load single pointer table
		ptrsInSinglePtrTable := VolumePtr(allocatedDataClustersInIndirect2(*inode, sb)) % getPtrsPerCluster(sb)
		if ptrsInSinglePtrTable == 0 {
			ptrsInSinglePtrTable = getPtrsPerCluster(sb)
		}

		singlePtrs := make([]ClusterPtr, ptrsInSinglePtrTable)
		err := volume.ReadStruct(ClusterPtrToVolumePtr(sb, doublePtrs[i]), singlePtrs)
		if err != nil {
			return allocatedSize, err
		}

		for j := len(singlePtrs) - 1; j >= 0; j-- {
			// Free data cluster
			err = FreeCluster(volume, sb, singlePtrs[j])
			if err != nil {
				return allocatedSize, err
			}

			if j == 0 {
				// Single pointer table is empty, let's free it
				err = FreeCluster(volume, sb, doublePtrs[i])
				if err != nil {
					return allocatedSize, err
				}
			}

			allocatedSize -= VolumePtr(sb.ClusterSize)
			inode.AllocatedClusters--
			clustersToBeDeallocated--

			if clustersToBeDeallocated <= 0 {
				stop = true
				break
			}
		}

		if stop {
			break
		}
	}

	if allocatedDataClustersInIndirect2(*inode, sb) <= 0 && inode.Indirect2 != Unused {
		// Deallocate double pointer table
		err = FreeCluster(volume, sb, inode.Indirect2)
		if err != nil {
			return allocatedSize, err
		}

		inode.Indirect2 = Unused
	}

	return allocatedSize, nil
}

func getPtrsPerCluster(sb Superblock) VolumePtr {
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
			err = volume.WriteStruct(InodePtrToVolumePtr(sb, inodePtr), inode)
			if err != nil {
				return VolumeObject{}, err
			}

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

func OccupyClusters(volume ReadWriteVolume, sb Superblock, clusterPtrs []ClusterPtr) error {
	for _, ptr := range clusterPtrs {
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
