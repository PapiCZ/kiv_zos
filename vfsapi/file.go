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
	offset       int
}

func Open(fs vfs.Filesystem, path string) (File, error) {
	mutableInode, err := GetInodeByPath(fs, *fs.RootInode, path)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			// Path doesn't exist, we want to create directory entry in parent inode
			pathFragments := strings.Split(path, "/")
			parentPath := pathFragments[:len(pathFragments)-1]
			name := pathFragments[len(pathFragments)-1]

			parentMutableInode, err := GetInodeByPath(fs, *fs.CurrentInode, strings.Join(parentPath, "/"))
			if err != nil {
				return File{}, err
			}

			// Create new file
			vo, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, true)
			if err != nil {
				return File{}, err
			}
			err = vfs.AppendDirectoryEntries(
				fs.Volume,
				fs.Superblock,
				parentMutableInode,
				vfs.NewDirectoryEntry(
					name,
					vfs.VolumePtrToInodePtr(fs.Superblock, vo.VolumePtr)),
			)
			if err != nil {
				return File{}, err
			}

			mutableInode, err = GetInodeByPath(fs, *fs.RootInode, path)
			if err != nil {
				return File{}, err
			}

		default:
			return File{}, err
		}
	}

	return File{
		filesystem:   fs,
		mutableInode: mutableInode,
		offset:       0,
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
		vfs.NewDirectoryEntry(
			name,
			vfs.VolumePtrToInodePtr(fs.Superblock, newDirInodeObj.VolumePtr)),
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
		vfs.NewDirectoryEntry(
			".",
			vfs.VolumePtrToInodePtr(fs.Superblock, newDirInodeObj.VolumePtr)),
		vfs.NewDirectoryEntry(
			"..",
			parentMutableInode.InodePtr),
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
	_, err = vfs.RemoveDirectoryEntry(fs.Volume, fs.Superblock, parentMutableInode, name)
	if err != nil {
		return err
	}

	return nil
}

func Rename(fs vfs.Filesystem, oldPath, newPath string) error {
	// Build variables for old path
	oldPathFragments := strings.Split(oldPath, "/")
	oldParentPath := oldPathFragments[:len(oldPathFragments)-1]
	oldName := oldPathFragments[len(oldPathFragments)-1]

	// Find old parent inode
	oldParentMutableInode, err := GetInodeByPath(fs, *fs.CurrentInode, strings.Join(oldParentPath, "/"))
	if err != nil {
		return err
	}

	// Remove directory entry from parent inode
	directoryEntry, err := vfs.RemoveDirectoryEntry(fs.Volume, fs.Superblock, oldParentMutableInode, oldName)
	if err != nil {
		return err
	}

	// Check if new path exists
	newMutableInode, err := GetInodeByPath(fs, *fs.CurrentInode, newPath)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			// New path doesn't exist, we want to add directory entry to parent inode
			newPathFragments := strings.Split(newPath, "/")
			newParentPath := newPathFragments[:len(newPathFragments)-1]
			newName := newPathFragments[len(newPathFragments)-1]

			newMutableInode, err = GetInodeByPath(fs, *fs.CurrentInode, strings.Join(newParentPath, "/"))
			if err != nil {
				return err
			}

			// Change name of directory entry
			directoryEntry.Name = vfs.StringNameToBytes(newName)
		default:
			return err
		}
	}

	err = vfs.AppendDirectoryEntries(fs.Volume, fs.Superblock, newMutableInode, directoryEntry)
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
			size:  int(mutableInode.Inode.Size),
			isDir: mutableInode.Inode.IsDir(),
		})
	}

	return fileInfos, nil
}

func (f *File) Write(data []byte) (int, error) {
	// TODO: Add offset param (or create new function (WriteData?))?
	n, err := f.mutableInode.AppendData(
		f.filesystem.Volume,
		f.filesystem.Superblock,
		data,
	)
	if err != nil {
		return int(n), err
	}

	f.offset += int(n)

	return int(n), nil
}

func (f *File) Read(data []byte) (int, error) {
	n, err := f.mutableInode.Inode.ReadData(
		f.filesystem.Volume,
		f.filesystem.Superblock,
		vfs.VolumePtr(f.offset),
		data,
	)
	if err != nil {
		return int(n), err
	}

	f.offset += int(n)

	return int(n), nil
}
