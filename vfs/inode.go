package vfs

const InodeIsFree = -1
const InodeDirectPointersCount = 5

type Inode struct {
	NodeId    int32
	Directory bool
	FileSize  Vptr
	Direct1   Cptr
	Direct2   Cptr
	Direct3   Cptr
	Direct4   Cptr
	Direct5   Cptr
	Indirect1 Cptr
	Indirect2 Cptr
}

func (i Inode) IsFree() bool {
	return i.NodeId == InodeIsFree
}

func(i *Inode) Free() {
	i.NodeId = InodeIsFree
}
