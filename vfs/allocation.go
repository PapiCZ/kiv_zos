package vfs

import "unsafe"

func Allocate(length int32) {

}

func FindFreeInode(volume Volume, superblock Superblock) (Vptr, Inode, error) {
	address := superblock.InodeStartAddress
	inodeStructSize := Vptr(unsafe.Sizeof(Inode{}))

	for address + inodeStructSize < superblock.DataStartAddress {
		inode := Inode{}
		err := volume.ReadStruct(address, &inode)
		if err != nil {
			return -1, Inode{}, err
		}

		if inode.IsFree() {
			return address, inode, nil
		}

		address += inodeStructSize
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

//func FreeAllInodes(volume Volume, superblock Superblock) error {
//	address := superblock.InodeStartAddress
//	inodeStructSize := int32(unsafe.Sizeof(Inode{}))
//
//	for address + inodeStructSize < superblock.DataStartAddress {
//		inode := Inode{}
//		err := volume.ReadStruct(address, &inode)
//		if err != nil {
//			return err
//		}
//	}
//
//	return -1, Inode{}, nil
//}
