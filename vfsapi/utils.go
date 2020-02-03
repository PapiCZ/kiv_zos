package vfsapi

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"strings"
)

func GetInodeByPath(fs vfs.Filesystem, currentMutableInode vfs.MutableInode, path string) (vfs.MutableInode, error) {
	pathFragments := strings.Split(path, "/")

	currentInode := currentMutableInode
	for _, pathFragment := range pathFragments {
		if len(pathFragment) == 0 {
			continue
		}

		_, directoryEntry, err := vfs.FindDirectoryEntryByName(fs.Volume, fs.Superblock, *currentInode.Inode, pathFragment)
		if err != nil {
			return vfs.MutableInode{}, err
		}

		mutableInode, err := vfs.LoadMutableInode(fs.Volume, fs.Superblock, directoryEntry.InodePtr)
		if err != nil {
			return vfs.MutableInode{}, err
		}

		currentInode = mutableInode
	}

	return vfs.MutableInode{
		Inode:    currentInode.Inode,
		InodePtr: currentInode.InodePtr,
	}, nil
}

func CToGoString(data []byte) string {
	n := -1
	for i, b := range data {
		if b == 0 {
			break
		}
		n = i
	}
	return string(data[:n+1])
}
