package vfs

import "unsafe"

func AlignNumber(number, multiplier int) int {
	return number + (multiplier - (number % multiplier))
}

func InodePtrToVolumePtr(superblock Superblock, ptr InodePtr) VolumePtr {
	return superblock.InodesStartAddress + VolumePtr(ptr * InodePtr(unsafe.Sizeof(Inode{})))
}

func VolumePtrToInodePtr(superblock Superblock, ptr VolumePtr) InodePtr {
	return InodePtr((ptr - superblock.InodesStartAddress) / VolumePtr(unsafe.Sizeof(Inode{})))
}
