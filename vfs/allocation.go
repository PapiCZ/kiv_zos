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

func Allocate(volume ReadWriteVolume, superblock Superblock, size VolumePtr) (Inode, VolumePtr, error) {
	// TODO: Maybe should return VolumeObject because caller doesn't know address of the inode
	// TODO: Do we have enough clusters and space?

	inodeObject, err := FindFreeInode(volume, superblock)
	if err != nil {
		return Inode{}, 0, err
	}

	inode := inodeObject.Object.(Inode)
	allocatedSize := VolumePtr(0)

	// Allocate direct blocks
	allocatedSizeDirect, err := AllocateDirect(&inode, volume, superblock, size)
	if err != nil {
		return Inode{}, 0, err
	}
	size -= allocatedSizeDirect
	allocatedSize += allocatedSizeDirect

	if size > 0 {
		// Allocate indirect1
		allocatedSizeIndirect1, err := AllocateIndirect1(&inode, volume, superblock, size)
		if err != nil {
			return Inode{}, 0, err
		}
		size -= allocatedSizeIndirect1
		allocatedSize += allocatedSizeIndirect1

		if size > 0 {
			// Allocate indirect2
			allocatedSizeIndirect2, err := AllocateIndirect2(&inode, volume, superblock, size)
			if err != nil {
				return Inode{}, 0, err
			}
			allocatedSize += allocatedSizeIndirect2
		}
	}

	inode.AllocatedClusters = ClusterPtr(allocatedSize / VolumePtr(superblock.ClusterSize))

	return inode, allocatedSize, nil
}

func AllocateDirect(inode *Inode, volume ReadWriteVolume, superblock Superblock, size VolumePtr) (VolumePtr, error) {
	volume = NewCachedVolume(volume)
	defer func() {
		_ = volume.(CachedVolume).Flush()
	}()

	directPtrs := [...]*ClusterPtr{
		&inode.Direct1,
		&inode.Direct2,
		&inode.Direct3,
		&inode.Direct4,
		&inode.Direct5,
	}

	allocatedSize := VolumePtr(0)
	if size > VolumePtr(len(directPtrs) * int(superblock.ClusterSize)) {
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
	volume = NewCachedVolume(volume)
	defer func() {
		_ = volume.(CachedVolume).Flush()
	}()

	var cp ClusterPtr
	clusterPtrSize := int(unsafe.Sizeof(cp))

	// We can allocate only clusterSize/clusterPtrSize * clusterSize bytes
	size = VolumePtr(math.Min(float64(size), float64(int(superblock.ClusterSize)/clusterPtrSize * int(superblock.ClusterSize))))

	// Find free cluster for pointers
	ptrClusterObjects, err := FindFreeClusters(volume, superblock, 1, true)
	if err != nil {
		return 0, err
	}
	ptrClusterObj := ptrClusterObjects[0]

	allocatedSize := VolumePtr(0)

	clusterPtrs := make([]ClusterPtr, 0)
	neededClusters := NeededClusters(superblock, size)
	dataClusterObjects, err := FindFreeClusters(volume, superblock, neededClusters, true)

	// Find clusters and store their addresses in ptrClusterObj
	for _, dataClusterObj := range dataClusterObjects {
		dataClusterPtr := VolumePtrToClusterPtr(superblock, dataClusterObj.VolumePtr)

		clusterPtrs = append(clusterPtrs, dataClusterPtr)
		allocatedSize += VolumePtr(superblock.ClusterSize)
	}

	// Fill remaining ptrs with -1
	for i := len(clusterPtrs); i < int(superblock.ClusterSize)/clusterPtrSize; i++ {
		clusterPtrs = append(clusterPtrs, Unused)
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
	volume = NewCachedVolume(volume)
	defer func() {
		_ = volume.(CachedVolume).Flush()
	}()

	doubleClusterPtrObjects, err := FindFreeClusters(volume, superblock, 1, true)
	if err != nil {
		return 0, err
	}
	doubleClusterPtrObj := doubleClusterPtrObjects[0]

	allocatedSize := VolumePtr(0)
	var cp ClusterPtr
	clusterPtrSize := int(unsafe.Sizeof(cp))
	neededDataClusters := NeededClusters(superblock, size)
	neededSinglePtrClusters := NeededClusters(superblock, VolumePtr(neededDataClusters)*VolumePtr(clusterPtrSize))

	singlePtrClusterObjects, err := FindFreeClusters(volume, superblock, neededSinglePtrClusters, true)
	if err != nil {
		return 0, err
	}
	dataClusterObjects, err := FindFreeClusters(volume, superblock, neededDataClusters, true)
	if err != nil {
		return 0, err
	}

	doublePtrs := make([]ClusterPtr, 0)
	dataClusterIterator := 0
	for _, singlePtrClusterObj := range singlePtrClusterObjects {
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

		// Fill remaining ptrs with -1
		for i := len(singlePtrs); i < int(superblock.ClusterSize)/clusterPtrSize; i++ {
			singlePtrs = append(singlePtrs, Unused)
		}

		singlePtrClusterObj.Object = singlePtrs
		err = singlePtrClusterObj.Save()
		if err != nil {
			// TODO: Free occupied clusters
			return 0, err
		}
	}

	// Fill remaining ptrs with -1
	for i := len(doublePtrs); i < int(superblock.ClusterSize)/clusterPtrSize; i++ {
		doublePtrs = append(doublePtrs, Unused)
	}

	doubleClusterPtrObj.Object = doublePtrs
	err = doubleClusterPtrObj.Save()
	if err != nil {
		// TODO: Free occupied clusters
		return 0, err
	}
	inode.Indirect2 = VolumePtrToClusterPtr(superblock, doubleClusterPtrObj.VolumePtr)

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
			inode := Inode{}
			err := volume.ReadStruct(InodePtrToVolumePtr(superblock, inodePtr), &inode)
			if err != nil {
				return VolumeObject{}, err
			}

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
		for clusterPtr := ClusterPtr(volumeOffset*8); clusterPtr < ClusterPtr(volumeOffset*8) + ClusterPtr(n*8); clusterPtr++ {
			value, err := bitmap.GetBit(VolumePtr(clusterPtr) - (volumeOffset*8))
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
					Object:    make([]byte, superblock.ClusterSize),
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
