package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"testing"
)

func TestDirectoryEntryCreation(t *testing.T) {
	fs := PrepareFS(1e7, t)

	rootInodeObj, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, true)
	if err != nil {
		t.Fatal(err)
	}
	rootInode := rootInodeObj.Object.(vfs.Inode)

	err = vfs.InitRootDirectory(fs.Volume, fs.Superblock, vfs.VolumePtrToInodePtr(fs.Superblock, rootInodeObj.VolumePtr), vfs.MutableInode{
		Inode:    &rootInode,
		InodePtr: vfs.VolumePtrToInodePtr(fs.Superblock, rootInodeObj.VolumePtr),
	})
	if err != nil {
		t.Fatal(err)
	}

	childInode, err := vfs.CreateNewDirectory(
		fs.Volume,
		fs.Superblock,
		vfs.VolumePtrToInodePtr(fs.Superblock, rootInodeObj.VolumePtr),
		vfs.MutableInode{
			Inode:    &rootInode,
			InodePtr: vfs.VolumePtrToInodePtr(fs.Superblock, rootInodeObj.VolumePtr),
		},
		"foobar",
	)
	if err != nil {
		t.Fatal(err)
	}

	// Check . in root inode
	deptr, directoryEntry, err := vfs.FindDirectoryEntryByName(fs.Volume, fs.Superblock, rootInode, ".")
	if err != nil {
		t.Fatal(err)
	}

	if deptr != 0 {
		t.Error("invalid directory entry pointer")
	}

	if directoryEntry.Name != vfs.StringNameToBytes(".") {
		t.Error("invalid directory name")
	}

	if directoryEntry.Inode != 0 {
		t.Error("invalid inode pointer in directory entry")
	}

	// Check .. in root inode
	deptr, directoryEntry, err = vfs.FindDirectoryEntryByName(fs.Volume, fs.Superblock, rootInode, "..")
	if err != nil {
		t.Fatal(err)
	}

	if deptr != 1 {
		t.Error("invalid directory entry pointer")
	}

	if directoryEntry.Name != vfs.StringNameToBytes("..") {
		t.Error("invalid directory name")
	}

	if directoryEntry.Inode != 0 {
		t.Error("invalid inode pointer in directory entry")
	}

	// Check foobar in root inode
	deptr, directoryEntry, err = vfs.FindDirectoryEntryByName(fs.Volume, fs.Superblock, rootInode, "foobar")
	if err != nil {
		t.Fatal(err)
	}

	if deptr != 2 {
		t.Error("invalid directory entry pointer")
	}

	if directoryEntry.Name != vfs.StringNameToBytes("foobar") {
		t.Error("invalid directory name")
	}

	if directoryEntry.Inode != 1 {
		t.Error("invalid inode pointer in directory entry")
	}

	// Check . in child inode
	deptr, directoryEntry, err = vfs.FindDirectoryEntryByName(fs.Volume, fs.Superblock, childInode, ".")
	if err != nil {
		t.Fatal(err)
	}

	if deptr != 0 {
		t.Error("invalid directory entry pointer")
	}

	if directoryEntry.Name != vfs.StringNameToBytes(".") {
		t.Error("invalid directory name")
	}

	if directoryEntry.Inode != 1 {
		t.Error("invalid inode pointer in directory entry")
	}

	// Check .. in child inode
	deptr, directoryEntry, err = vfs.FindDirectoryEntryByName(fs.Volume, fs.Superblock, childInode, "..")
	if err != nil {
		t.Fatal(err)
	}

	if deptr != 1 {
		t.Error("invalid directory entry pointer")
	}

	if directoryEntry.Name != vfs.StringNameToBytes("..") {
		t.Error("invalid directory name")
	}

	if directoryEntry.Inode != 0 {
		t.Error("invalid inode pointer in directory entry")
	}
}
