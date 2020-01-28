package vfs

import "unsafe"

func InodePtrToVolumePtr(sb Superblock, ptr InodePtr) VolumePtr {
	return sb.InodesStartAddress + VolumePtr(ptr * InodePtr(unsafe.Sizeof(Inode{})))
}

func VolumePtrToInodePtr(sb Superblock, ptr VolumePtr) InodePtr {
	return InodePtr((ptr - sb.InodesStartAddress) / VolumePtr(unsafe.Sizeof(Inode{})))
}

func ClusterPtrToVolumePtr(sb Superblock, ptr ClusterPtr) VolumePtr {
	return sb.DataStartAddress + VolumePtr(ptr * ClusterPtr(sb.ClusterSize))
}

func VolumePtrToClusterPtr(sb Superblock, ptr VolumePtr) ClusterPtr {
	return ClusterPtr((ptr - sb.DataStartAddress) / VolumePtr(sb.ClusterSize))
}

func ConvertByteToClusterPtr(data []byte) ClusterPtr {
	var clusterPtr ClusterPtr
	for i := 0; i < int(unsafe.Sizeof(ClusterPtr(0))); i++ {
		clusterPtr |= ClusterPtr(data[i]) << (i * 8)
	}

	return clusterPtr
}
