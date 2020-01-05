package vfs

import (
	"errors"
	"math"
	"unsafe"
)

const (
	FreeCluster = 0
	OccupiedCluster = 1
)

func Allocate(length Vptr, bitmap *Bitmap, superblock Superblock) {
	neededClusters, neededInodes := NeededClustersAndInodes(length, superblock)
}

func NeededClustersAndInodes(length Vptr, superblock Superblock) (Cptr, int) {
	neededClusters := Cptr(math.Ceil(float64(length) / float64(superblock.ClusterSize)))
	neededInodes := int(math.Ceil(float64(neededClusters) / InodeDirectPointersCount))

	return neededClusters, neededInodes
}

func FindFreeInode(volume Volume, superblock Superblock) (Vptr, Inode, error) {
	address := superblock.InodeStartAddress
	inodeSize := Vptr(unsafe.Sizeof(Inode{}))

	for address+inodeSize < superblock.DataStartAddress {
		inode := Inode{}
		err := volume.ReadStruct(address, &inode)
		if err != nil {
			return -1, Inode{}, err
		}

		if inode.IsFree() {
			return address, inode, nil
		}

		address += inodeSize
	}

	return -1, Inode{}, nil
}

func FreeInode(address Vptr, volume Volume) error {
	inode := Inode{}
	err := volume.ReadStruct(address, &inode)
	if err != nil {
		return err
	}

	inode.Free()
	err = volume.WriteStruct(address, &inode)
	if err != nil {
		return err
	}

	return nil
}

func FreeAllInodes(volume Volume, superblock Superblock) error {
	address := superblock.InodeStartAddress
	inodeSize := Vptr(unsafe.Sizeof(Inode{}))

	for address+inodeSize < superblock.DataStartAddress {
		err := FreeInode(address, volume)
		if err != nil {
			return err
		}

		address += inodeSize
	}

	return nil
}

func FindFreeClusters(bitmap Bitmap, count Vptr) ([]Vptr, error) {
	clusters := make([]Vptr, 0)

	for i, bit := range bitmap {
		if bit == FreeCluster {
			clusters = append(clusters, Vptr(i))

			if Vptr(len(clusters)) == count {
				return clusters, nil
			}
		}
	}

	return nil, errors.New("not enough available clusters")
}

func FindFreeInodes(volume Volume, superblock Superblock) int {
	inodes := make(map[Vptr]VolumeObject)
}
