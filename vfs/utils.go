package vfs

import "unsafe"

func InodePtrToVolumePtr(superblock Superblock, ptr InodePtr) VolumePtr {
	return superblock.InodesStartAddress + VolumePtr(ptr * InodePtr(unsafe.Sizeof(Inode{})))
}

func VolumePtrToInodePtr(superblock Superblock, ptr VolumePtr) InodePtr {
	return InodePtr((ptr - superblock.InodesStartAddress) / VolumePtr(unsafe.Sizeof(Inode{})))
}

func ClusterPtrToVolumePtr(superblock Superblock, ptr ClusterPtr) VolumePtr {
	return superblock.DataStartAddress + VolumePtr(ptr * ClusterPtr(superblock.ClusterSize))
}

func VolumePtrToClusterPtr(superblock Superblock, ptr VolumePtr) ClusterPtr {
	return ClusterPtr((ptr - superblock.DataStartAddress) / VolumePtr(superblock.ClusterSize))
}
