package vfs

import (
	"errors"
	"math"
)

const (
	FreeCluster     = 0
	OccupiedCluster = 1
)

type NoFreeInodeAvailableError struct{}

func (n NoFreeInodeAvailableError) Error() string {
	return "no free inode is aivailable"
}

//func Allocate(length VolumePtr, bitmap *Bitmap, superblock Superblock) {
//	neededClusters, neededInodes := NeededClustersAndInodes(length, superblock)
//}

func NeededClustersAndInodes(length VolumePtr, superblock Superblock) (ClusterPtr, int) {
	neededClusters := ClusterPtr(math.Ceil(float64(length) / float64(superblock.ClusterSize)))
	neededInodes := int(math.Ceil(float64(neededClusters) / InodeDirectPointersCount))

	return neededClusters, neededInodes
}

func FindFreeInode(volume Volume, superblock Superblock) (VolumeObject, error) {
	address := superblock.InodesStartAddress

	for inodePtr := InodePtr(0); true; inodePtr++ {
		isFree, err := IsInodeFree(volume, superblock, inodePtr)
		if err != nil {
			return VolumeObject{}, err
		}

		if isFree {
			inode := Inode{}
			err := volume.ReadStruct(address, &inode)
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

func FindFreeClusters(bitmap Bitmap, count VolumePtr) ([]VolumePtr, error) {
	clusters := make([]VolumePtr, 0)

	for i, bit := range bitmap {
		if bit == FreeCluster {
			clusters = append(clusters, VolumePtr(i))

			if VolumePtr(len(clusters)) == count {
				return clusters, nil
			}
		}
	}

	return nil, errors.New("not enough available clusters")
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

	return GetBitInByte(data, int8(ptr%8)) == 0, nil
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

func FreeInode(volume Volume, superblock Superblock, ptr InodePtr) error {
	return setValueInInodeBitmap(volume, superblock, ptr, 0)
}

func OccupyInode(volume Volume, superblock Superblock, ptr InodePtr) error {
	return setValueInInodeBitmap(volume, superblock, ptr, 1)
}
