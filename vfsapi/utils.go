package vfsapi

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"strings"
)

func getInodeByPathRecursively(fs vfs.Filesystem, path string) (vfs.MutableInode, error) {
	if len(path) >= 1 && path[0] == '/' {
		// Absolute path
		return getInodeByPathFromInodeRecursively(fs, fs.RootInodePtr, path)
	} else {
		// Relative path
		return getInodeByPathFromInodeRecursively(fs, fs.CurrentInodePtr, path)
	}
}

func getInodeByPathFromInodeRecursively(fs vfs.Filesystem, currentInodePtr vfs.InodePtr, path string) (vfs.MutableInode, error) {
	pathFragments := strings.Split(path, "/")

	currentMutableInode, err := vfs.LoadMutableInode(fs.Volume, fs.Superblock, currentInodePtr)
	if err != nil {
		return vfs.MutableInode{}, err
	}

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

func cToGoString(data []byte) string {
	n := -1
	for i, b := range data {
		if b == 0 {
			break
		}
		n = i
	}
	return string(data[:n+1])
}

func splitString(s string, sep string) []string {
	absolute := false
	if s[0] == '/' {
		absolute = true
	}

	if absolute {
		return append([]string{""}, strings.Split(s, sep)...)
	}

	return strings.Split(s, sep)
}

func joinString(fragments []string, sep string) string {
	return strings.Join(fragments, sep)
}
