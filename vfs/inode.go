package vfs

const InodeFree = -1

type Inode struct {
	nodeId    int32
	directory bool
	fileSize  int32
	direct1   int32
	direct2   int32
	direct3   int32
	direct4   int32
	direct5   int32
	indirect1 int32
	indirect2 int32
}

func (i Inode) IsFree() bool {
	return i.nodeId == InodeFree
}
