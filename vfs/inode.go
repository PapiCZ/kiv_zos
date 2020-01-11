package vfs

const InodeIsFree = -1
const InodeDirectPointersCount = 5

type Inode struct {
	Directory bool
	FileSize  VolumePtr
	Direct1   ClusterPtr
	Direct2   ClusterPtr
	Direct3   ClusterPtr
	Direct4   ClusterPtr
	Direct5   ClusterPtr
	Indirect1 ClusterPtr
	Indirect2 ClusterPtr
}

