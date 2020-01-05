package vfs

const InodeIsFree = -1

type Inode struct {
	NodeId    int32
	Directory bool
	FileSize  Vptr
	Direct1   Vptr
	Direct2   Vptr
	Direct3   Vptr
	Direct4   Vptr
	Direct5   Vptr
	Indirect1 Vptr
	Indirect2 Vptr
}

func (i Inode) IsFree() bool {
	return i.NodeId == InodeIsFree
}

func(i *Inode) Free() {
	i.NodeId = InodeIsFree
}
