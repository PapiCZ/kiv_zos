package vfs

import "unsafe"

func Allocate(length int32) {

}

func FindFreeInode(volume Volume, superblock Superblock) (Inode, error) {
	address := superblock.InodeStartAddress
	inodeStructSize := int32(unsafe.Sizeof(Inode{}))

	for address + inodeStructSize < superblock.DataStartAddress {
		inode := Inode{}
		err := volume.ReadStruct(address, &inode)
		if err != nil {
			return Inode{}, err
		}

		if inode.IsFree() {
			return inode, nil
		}
	}

	return Inode{}, nil
}
