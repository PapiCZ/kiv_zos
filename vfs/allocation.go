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

	inode.AllocatedClusters += ClusterPtr(allocatedSize / VolumePtr(superblock.ClusterSize))

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
	}

	return allocatedSize, nil
}

func AllocateIndirect1(inode *Inode, volume ReadWriteVolume, superblock Superblock, size VolumePtr) (VolumePtr, error) {
	var err error

	//volume = NewCachedVolume(volume)
	//defer func() {
	//	_ = volume.(CachedVolume).Flush()
	//}()

	var cp ClusterPtr
	clusterPtrSize := int(unsafe.Sizeof(cp))

	// We can allocate only clusterSize/clusterPtrSize * clusterSize bytes
	maxSize := int(superblock.ClusterSize) / clusterPtrSize * int(superblock.ClusterSize)
	size = VolumePtr(math.Min(float64(size), float64(maxSize)))
	if size <= 0 {
		return 0, nil
	}

	// Find free cluster for pointers
	var ptrClusterObj VolumeObject
	if inode.Indirect1 == Unused {
		ptrClusterObjects, err := FindFreeClusters(volume, superblock, 1, true)
		if err != nil {
			return 0, err
		}
		ptrClusterObj = ptrClusterObjects[0]
		ptrClusterObj.Object = make([]ClusterPtr, 0)
	} else {
		ptrsPerCluster := VolumePtr(superblock.ClusterSize) / VolumePtr(clusterPtrSize)
		alreadyAllocatedClusters := VolumePtr(inode.AllocatedClusters-InodeDirectCount) *
			VolumePtr(superblock.ClusterSize) / VolumePtr(superblock.ClusterSize)

		if alreadyAllocatedClusters >= ptrsPerCluster {
			// We can't allocate more clusters in indirect1
			return 0, nil
		}

		data := make([]ClusterPtr, alreadyAllocatedClusters)
		ptrClusterObj, err = volume.ReadObject(ClusterPtrToVolumePtr(superblock, inode.Indirect1), data)
		if err != nil {
			return 0, err
		}
	}

	allocatedSize := VolumePtr(0)

	clusterPtrs := ptrClusterObj.Object.([]ClusterPtr)
	neededClusters := NeededClusters(superblock, size)
	dataClusterObjects, err := FindFreeClusters(volume, superblock, neededClusters, true)

	// Find clusters and store their addresses in ptrClusterObj
	for _, dataClusterObj := range dataClusterObjects {
		dataClusterPtr := VolumePtrToClusterPtr(superblock, dataClusterObj.VolumePtr)

		clusterPtrs = append(clusterPtrs, dataClusterPtr)
		allocatedSize += VolumePtr(superblock.ClusterSize)
	}

	ptrClusterObj.Object = clusterPtrs
	err = ptrClusterObj.Save()
	if err != nil {
		// TODO: Free occupied clusters
		return 0, nil
	}
	inode.Indirect1 = VolumePtrToClusterPtr(superblock, ptrClusterObj.VolumePtr)

	return allocatedSize, nil
}

func AllocateIndirect2(inode *Inode, volume ReadWriteVolume, superblock Superblock, size VolumePtr) (VolumePtr, error) {
	var err error

	//volume = NewCachedVolume(volume)
	//defer func() {
	//	_ = volume.(CachedVolume).Flush()
	//}()

	var cp ClusterPtr
	clusterPtrSize := int(unsafe.Sizeof(cp))

	// We can allocate only (clusterSize/clusterPtrSize)^2 * clusterSize bytes
	maxSize := int(math.Pow(float64(int(superblock.ClusterSize)/clusterPtrSize), 2)) * int(superblock.ClusterSize)
	size = VolumePtr(math.Min(float64(size), float64(maxSize)))
	if size <= 0 {
		return 0, nil
	}

	ptrsPerCluster := VolumePtr(superblock.ClusterSize) / VolumePtr(clusterPtrSize)

	var doublePtrsClusterObj VolumeObject
	if inode.Indirect2 == Unused {
		doublePtrClusterObjects, err := FindFreeClusters(volume, superblock, 1, true)
		if err != nil {
			return 0, err
		}
		doublePtrsClusterObj = doublePtrClusterObjects[0]
		doublePtrsClusterObj.Object = make([]ClusterPtr, 0)
	} else {
		alreadyAllocatedDoublePtrs := ClusterPtr(math.Ceil(
			float64(VolumePtr(inode.AllocatedClusters)-InodeDirectCount-ptrsPerCluster) / float64(superblock.ClusterSize),
		))
		data := make([]ClusterPtr, alreadyAllocatedDoublePtrs)
		doublePtrsClusterObj, err = volume.ReadObject(ClusterPtrToVolumePtr(superblock, inode.Indirect2), data)
		if err != nil {
			return 0, err
		}
	}

	// Load single pointer clusters
	singlePtrsClusterObjects := make([]VolumeObject, 0)
	doublePtrsClusterObjLength := len(doublePtrsClusterObj.Object.([]ClusterPtr))
	for i, doublePtr := range doublePtrsClusterObj.Object.([]ClusterPtr) {
		allocated := ptrsPerCluster
		if i == doublePtrsClusterObjLength-1 {
			// Last single pointer cluster is not fully filled
			allocated = (VolumePtr(inode.AllocatedClusters) - InodeDirectCount - ptrsPerCluster) % ptrsPerCluster
		}

		data := make([]ClusterPtr, allocated) // TODO: alreadyAllocatedSinglePtrs :=
		singlePtrsClusterObj, err := volume.ReadObject(ClusterPtrToVolumePtr(superblock, doublePtr), data)
		if err != nil {
			return 0, err
		}
		singlePtrsClusterObjects = append(singlePtrsClusterObjects, singlePtrsClusterObj)
	}

	allocatedSize := VolumePtr(0)

	neededDataClusters := NeededClusters(superblock, size)
	neededSinglePtrClusters := NeededClusters(superblock, VolumePtr(neededDataClusters)*VolumePtr(clusterPtrSize))
	newSinglePtrsClusterObjects, err := FindFreeClusters(volume, superblock, neededSinglePtrClusters, true)
	if err != nil {
		return 0, err
	}

	// Load data clusters
	dataClusterObjects := make([]VolumeObject, 0)
	for _, singlePtrs := range singlePtrsClusterObjects {
		for _, singlePtr := range singlePtrs.Object.([]ClusterPtr) {
			// We don't need real data (it would be very expensive)
			dataClusterObjects = append(dataClusterObjects,
				NewVolumeObject(ClusterPtrToVolumePtr(superblock, singlePtr), volume, nil))
		}
	}

	newDataClusterObjects, err := FindFreeClusters(volume, superblock, neededDataClusters, true)
	if err != nil {
		return 0, err
	}
	inode.AllocatedClusters += ClusterPtr(len(newDataClusterObjects))

	singlePtrsClusterObjects = append(singlePtrsClusterObjects, newSinglePtrsClusterObjects...)
	dataClusterObjects = append(dataClusterObjects, newDataClusterObjects...)

	doublePtrs := make([]ClusterPtr, 0)
	dataClusterIterator := 0
	for _, singlePtrClusterObj := range singlePtrsClusterObjects {
		singlePtrClusterPtr := VolumePtrToClusterPtr(superblock, singlePtrClusterObj.VolumePtr)

		doublePtrs = append(doublePtrs, singlePtrClusterPtr)
		singlePtrs := make([]ClusterPtr, 0)
		for i := 0; i < int(superblock.ClusterSize)/clusterPtrSize; i++ {
			if dataClusterIterator >= len(dataClusterObjects) {
				break
			}

			dataClusterPtr := VolumePtrToClusterPtr(superblock, dataClusterObjects[dataClusterIterator].VolumePtr)

			singlePtrs = append(singlePtrs, dataClusterPtr)
			allocatedSize += VolumePtr(superblock.ClusterSize)

			dataClusterIterator++
		}

		singlePtrClusterObj.Object = singlePtrs
		err = singlePtrClusterObj.Save()
		if err != nil {
			// TODO: Free occupied clusters
			return 0, err
		}
	}

	doublePtrsClusterObj.Object = doublePtrs
	err = doublePtrsClusterObj.Save()
	if err != nil {
		// TODO: Free occupied clusters
		return 0, err
	}
	inode.Indirect2 = VolumePtrToClusterPtr(superblock, doublePtrsClusterObj.VolumePtr)

	if allocatedSize < size {
		// TODO: Free occupied clusters
		return 0, errors.New("can't allocate requested space")
	}

	return allocatedSize, nil
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

func FreeClustersInIndirect1(inode Inode, superblock Superblock) ClusterPtr {
	// Subtract direct ptrs
	allocatedClusters := inode.AllocatedClusters - 5

	var cp ClusterPtr
	clusterPtrSize := int(unsafe.Sizeof(cp))
	maxClustersIndirect1 := int(superblock.ClusterSize) / clusterPtrSize

	return ClusterPtr(math.Max(float64(ClusterPtr(maxClustersIndirect1)-allocatedClusters), 0))
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

				clusterObjects = append(clusterObjects, VolumeObject{
					VolumePtr: ClusterPtrToVolumePtr(superblock, clusterPtr),
					Volume:    volume,
					Object:    nil,
				})
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
		n = superblock.InodeBitmapStartAddress - superblock.ClusterBitmapStartAddress - 1
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
