package vfs

import "unsafe"

const (
	Free     = 0
	Occupied = 1
)

type NoFreeInodeAvailableError struct{}

func (n NoFreeInodeAvailableError) Error() string {
	return "no free inode is available"
}

type NoFreeClusterAvailableError struct{}

func (n NoFreeClusterAvailableError) Error() string {
	return "no free cluster is available"
}

func Allocate(volume Volume, superblock Superblock, length VolumePtr) (VolumePtr, error) {
	// TODO: Do we have enough clusters and space?

	inodeObject, err := FindFreeInode(volume, superblock)
	if err != nil {
		return 0, err
	}

	inode := inodeObject.Object.(Inode)

	// Allocate direct blocks
	allocatedLength, err := AllocateDirect(&inode, volume, superblock, length)
	if err != nil {
		return allocatedLength, err
	}
	length -= allocatedLength

	if length > 0 {

	}

	return 0, nil
}

func AllocateDirect(inode *Inode, volume Volume, superblock Superblock, length VolumePtr) (VolumePtr, error) {
	directPtrs := [...]*ClusterPtr{
		&inode.Direct1,
		&inode.Direct2,
		&inode.Direct3,
		&inode.Direct4,
		&inode.Direct5,
	}

	allocatedLength := VolumePtr(0)

	// Find clusters for direct pointers
	for _, directPtr := range directPtrs {
		clusterObj, err := FindFreeCluster(volume, superblock)
		if err != nil {
			// TODO: Free occupied clusters
			return 0, err
		}

		clusterPtr := VolumePtrToClusterPtr(superblock, clusterObj.VolumePtr)
		err = OccupyCluster(volume, superblock, clusterPtr)
		if err != nil {
			// TODO: Free occupied clusters
			return 0, err
		}
		*directPtr = clusterPtr
		allocatedLength += VolumePtr(superblock.ClusterSize)

		if length-allocatedLength <= 0 {
			break
		}
	}

	return allocatedLength, nil
}

func AllocateIndirect1(inode *Inode, volume Volume, superblock Superblock, length VolumePtr) (VolumePtr, error) {
	// Find free cluster for pointers
	ptrClusterObj, err := FindFreeCluster(volume, superblock)
	if err != nil {
		return 0, err
	}

	err = OccupyCluster(volume, superblock, VolumePtrToClusterPtr(superblock, ptrClusterObj.VolumePtr))
	if err != nil {
		return 0, nil
	}

	allocatedLength := VolumePtr(0)
	var vp VolumePtr
	clusterPtrSize := int(unsafe.Sizeof(vp))
	clusterPtrs := make([]ClusterPtr, 0)

	// Find clusters and store their addresses in ptrClusterObj
	for i := 0; i < int(superblock.ClusterSize)/clusterPtrSize; i++ {
		dataClusterObj, err := FindFreeCluster(volume, superblock)
		if err != nil {
			// TODO: Free occupied clusters
			return 0, err
		}

		dataClusterPtr := VolumePtrToClusterPtr(superblock, dataClusterObj.VolumePtr)
		err = OccupyCluster(volume, superblock, dataClusterPtr)
		if err != nil {
			// TODO: Free occupied clusters
			return 0, nil
		}

		clusterPtrs = append(clusterPtrs, dataClusterPtr)
		allocatedLength += VolumePtr(superblock.ClusterSize)

		if length-allocatedLength <= 0 {
			break
		}
	}

	ptrClusterObj.Object = clusterPtrs
	err = ptrClusterObj.Save()
	if err != nil {
		// TODO: Free occupied clusters
		return 0, nil
	}
	inode.Indirect1 = VolumePtrToClusterPtr(superblock, ptrClusterObj.VolumePtr)

	return allocatedLength, nil
}

func AllocateIndirect2(inode *Inode, volume Volume, superblock Superblock, length VolumePtr) (VolumePtr, error) {
	ptrClusterObj1, err := FindFreeCluster(volume, superblock)
	if err != nil {
		return 0, err
	}

	err = OccupyCluster(volume, superblock, VolumePtrToClusterPtr(superblock, ptrClusterObj1.VolumePtr))
	if err != nil {
		return 0, nil
	}

	allocatedLength := VolumePtr(0)
	var vp VolumePtr
	clusterPtrSize := int(unsafe.Sizeof(vp))
	clusterPtrs1 := make([]ClusterPtr, 0)

	// Find clusters and store their addresses in ptrClusterObj1
	for i := 0; i < int(superblock.ClusterSize)/clusterPtrSize; i++ {
		ptrClusterObj2, err := FindFreeCluster(volume, superblock)
		if err != nil {
			// TODO: Free occupied clusters
			return 0, err
		}

		ptrCluster2 := VolumePtrToClusterPtr(superblock, ptrClusterObj2.VolumePtr)
		err = OccupyCluster(volume, superblock, ptrCluster2)
		if err != nil {
			// TODO: Free occupied clusters
			return 0, nil
		}

		clusterPtrs1 = append(clusterPtrs1, ptrCluster2)
		clusterPtrs2 := make([]ClusterPtr, 0)

		// Find clusters and store their addresses in ptrClusterObj2
		for j := 0; j < int(superblock.ClusterSize)/clusterPtrSize; j++ {
			dataClusterObj, err := FindFreeCluster(volume, superblock)
			if err != nil {
				// TODO: Free occupied clusters
				return 0, err
			}

			dataClusterPtr := VolumePtrToClusterPtr(superblock, dataClusterObj.VolumePtr)
			err = OccupyCluster(volume, superblock, dataClusterPtr)
			if err != nil {
				// TODO: Free occupied clusters
				return 0, nil
			}

			clusterPtrs2 = append(clusterPtrs2, dataClusterPtr)
			allocatedLength += VolumePtr(superblock.ClusterSize)

			if length-allocatedLength <= 0 {
				break
			}
		}

		ptrClusterObj2.Object = clusterPtrs2
		err = ptrClusterObj2.Save()
		if err != nil {
			// TODO: Free occupied clusters
			return 0, nil
		}

		if length-allocatedLength <= 0 {
			break
		}
	}

	ptrClusterObj1.Object = clusterPtrs1
	err = ptrClusterObj1.Save()
	if err != nil {
		// TODO: Free occupied clusters
		return 0, nil
	}
	inode.Indirect2 = VolumePtrToClusterPtr(superblock, ptrClusterObj1.VolumePtr)

	return allocatedLength, nil
}

func FindFreeInode(volume Volume, superblock Superblock) (VolumeObject, error) {
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

func IsInodeFree(volume Volume, superblock Superblock, ptr InodePtr) (bool, error) {
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

func FreeAllInodes(volume Volume, superblock Superblock) error {
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

func setValueInInodeBitmap(volume Volume, superblock Superblock, ptr InodePtr, value byte) error {
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

func OccupyInode(volume Volume, superblock Superblock, ptr InodePtr) error {
	return setValueInInodeBitmap(volume, superblock, ptr, Occupied)
}

func FreeInode(volume Volume, superblock Superblock, ptr InodePtr) error {
	return setValueInInodeBitmap(volume, superblock, ptr, Free)
}

func FindFreeCluster(volume Volume, superblock Superblock) (VolumeObject, error) {
	for inodePtr := ClusterPtr(0); true; inodePtr++ {
		isFree, err := IsClusterFree(volume, superblock, inodePtr)
		if err != nil {
			return VolumeObject{}, err
		}

		if isFree {
			cluster := make([]byte, superblock.ClusterSize)
			err := volume.ReadStruct(ClusterPtrToVolumePtr(superblock, inodePtr), &cluster)
			if err != nil {
				return VolumeObject{}, err
			}

			return NewVolumeObject(
				ClusterPtrToVolumePtr(superblock, inodePtr),
				volume,
				cluster,
			), nil
		}
	}

	return VolumeObject{}, NoFreeClusterAvailableError{}
}

func IsClusterFree(volume Volume, superblock Superblock, ptr ClusterPtr) (bool, error) {
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

func FreeAllClusters(volume Volume, superblock Superblock) error {
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

func setValueInClusterBitmap(volume Volume, superblock Superblock, ptr ClusterPtr, value byte) error {
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

func OccupyCluster(volume Volume, superblock Superblock, ptr ClusterPtr) error {
	return setValueInClusterBitmap(volume, superblock, ptr, Occupied)
}

func OccupyClusters(volume Volume, superblock Superblock, ptrs []ClusterPtr) error {
	for _, ptr := range ptrs {
		err := OccupyCluster(volume, superblock, ptr)
		if err != nil {
			return err
		}
	}

	return nil
}

func FreeCluster(volume Volume, superblock Superblock, ptr ClusterPtr) error {
	return setValueInClusterBitmap(volume, superblock, ptr, Free)
}
