package vfsapi

import (
	"fmt"
	"github.com/PapiCZ/kiv_zos/vfs"
	"strings"
)

type DirectoryIsNotEmpty struct {
	Name string
}

func (d DirectoryIsNotEmpty) Error() string {
	return fmt.Sprintf("directory %s is not empty", d.Name)
}

type File struct {
	filesystem   vfs.Filesystem
	mutableInode vfs.MutableInode
	ptrOffset    int64
}

func Open(fs vfs.Filesystem, name string) (File, error) {
	mutableInode, err := GetInodeByPath(fs, *fs.RootInode, name)
	if err != nil {
		return File{}, err
	}

	return File{
		filesystem:   fs,
		mutableInode: mutableInode,
		ptrOffset:    0,
	}, nil
}

func Mkdir(fs vfs.Filesystem, path string) error {
	pathFragments := strings.Split(path, "/")
	parentPath := pathFragments[:len(pathFragments)-1]
	name := pathFragments[len(pathFragments)-1]

	parentMutableInode, err := GetInodeByPath(fs, *fs.CurrentInode, strings.Join(parentPath, "/"))
	if err != nil {
		return err
	}

	newDirInodeObj, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, true)
	if err != nil {
		return err
	}
	newDirInode := newDirInodeObj.Object.(vfs.Inode)
	newDirInode.Type = vfs.InodeDirectoryType
	newDirInodeObj.Object = newDirInode
	err = newDirInodeObj.Save()
	if err != nil {
		return err
	}

	// Create new directory in parent newDirInode
	err = vfs.AppendDirectoryEntries(
		fs.Volume,
		fs.Superblock,
		parentMutableInode,
		[]vfs.DirectoryEntry{
			vfs.NewDirectoryEntry(
				name,
				vfs.VolumePtrToInodePtr(fs.Superblock, newDirInodeObj.VolumePtr)),
		},
	)
	if err != nil {
		return err
	}

	// Initialize newly created directory (add . and ..)
	err = vfs.AppendDirectoryEntries(
		fs.Volume,
		fs.Superblock,
		vfs.MutableInode{
			Inode:    &newDirInode,
			InodePtr: vfs.VolumePtrToInodePtr(fs.Superblock, newDirInodeObj.VolumePtr),
		},
		[]vfs.DirectoryEntry{
			vfs.NewDirectoryEntry(
				".",
				vfs.VolumePtrToInodePtr(fs.Superblock, newDirInodeObj.VolumePtr)),
			vfs.NewDirectoryEntry(
				"..",
				parentMutableInode.InodePtr),
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func Remove(fs vfs.Filesystem, path string) error {
	pathFragments := strings.Split(path, "/")
	parentPath := pathFragments[:len(pathFragments)-1]
	name := pathFragments[len(pathFragments)-1]

	parentMutableInode, err := GetInodeByPath(fs, *fs.CurrentInode, strings.Join(parentPath, "/"))
	if err != nil {
		return err
	}

	// Free inode
	fileMutableInode, err := GetInodeByPath(fs, *fs.CurrentInode, path)
	if err != nil {
		return err
	}

	// Check if file is dir and empty
	if fileMutableInode.Inode.IsDir() {
		directoryEntries, err := vfs.ReadAllDirectoryEntries(fs.Volume, fs.Superblock, *fileMutableInode.Inode)
		if err != nil {
			return err
		}

		// Empty directory contains 2 directory entries (. and ..)
		if len(directoryEntries) != 2 {
			return DirectoryIsNotEmpty{Name: path}
		}
	}

	err = vfs.FreeInode(fs.Volume, fs.Superblock, fileMutableInode.InodePtr)
	if err != nil {
		return err
	}

	// Remove directory entry
	err = vfs.RemoveDirectoryEntry(fs.Volume, fs.Superblock, parentMutableInode, name)
	if err != nil {
		return err
	}

	return nil
}

func (f File) ReadDir() ([]FileInfo, error) {
	fileInfos := make([]FileInfo, 0)

	directoryEntries, err := vfs.ReadAllDirectoryEntries(f.filesystem.Volume, f.filesystem.Superblock, *f.mutableInode.Inode)
	if err != nil {
		return fileInfos, err
	}

	for _, directoryEntry := range directoryEntries {
		mutableInode, err := vfs.LoadMutableInode(f.filesystem.Volume, f.filesystem.Superblock, directoryEntry.InodePtr)
		if err != nil {
			return fileInfos, err
		}
		fileInfos = append(fileInfos, FileInfo{
			name:  CToGoString(directoryEntry.Name[:]),
			size:  int64(mutableInode.Inode.Size),
			isDir: mutableInode.Inode.IsDir(),
		})
	}

	return fileInfos, nil
}
