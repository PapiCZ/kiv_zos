package vfs

type DirectoryEntry struct {
	name  [12]byte
	inode InodePtr
}
