package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"testing"
)

func PrepareFS(size vfs.VolumePtr, t *testing.T) vfs.Filesystem {
	// Create volume
	path := tempFileName("", "")
	err := vfs.PrepareVolumeFile(path, size)

	volume, err := vfs.NewVolume(path)
	if err != nil {
		t.Fatal(err)
	}

	// Create filesystem
	fs, err := vfs.NewFilesystem(volume, 2048)
	if err != nil {
		t.Fatal(err)
	}

	err = fs.WriteStructureToVolume()
	if err != nil {
		t.Fatal(err)
	}

	// Check written superblock
	s := vfs.Superblock{}
	err = volume.ReadStruct(0, &s)
	if err != nil {
		t.Fatal(err)
	}

	vo, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, false)
	if err != nil {
		t.Fatal(err)
	}

	if vo.VolumePtr != s.InodesStartAddress {
		t.Errorf("incorrect inode address: %d instead of %d", vo.VolumePtr, s.InodesStartAddress)
	}

	result, err := vfs.IsInodeFree(vo.Volume, s, vfs.VolumePtrToInodePtr(s, vo.VolumePtr))
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Errorf("inode is not free")
	}

	return vfs.Filesystem{
		Volume:     volume,
		Superblock: s,
	}
}

func TestAllocate(t *testing.T) {
	fs := PrepareFS(1e9, t)
	defer func() {
		_ = fs.Volume.Destroy()
	}()

	inodeObject, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, true)
	if err != nil {
		t.Fatal(err)
	}

	inode := inodeObject.Object.(vfs.Inode)
	allocatedSize, err := vfs.Allocate(vfs.MutableInode{
		Inode:    &inode,
		InodePtr: vfs.VolumePtrToInodePtr(fs.Superblock, inodeObject.VolumePtr),
	}, fs.Volume, fs.Superblock, 1e8)
	if err != nil {
		t.Fatal(err)
	}

	if allocatedSize != 48829*vfs.VolumePtr(fs.Superblock.ClusterSize) {
		t.Errorf("allocated incorrect size, %d instead of %d", allocatedSize, 48829*vfs.VolumePtr(fs.Superblock.ClusterSize))
	}
}

func TestReallocation(t *testing.T) {
	fs := PrepareFS(1e9, t)
	defer func() {
		_ = fs.Volume.Destroy()
	}()

	inodeObject, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, true)
	if err != nil {
		t.Fatal(err)
	}

	inode := inodeObject.Object.(vfs.Inode)
	expectedAllocationSize := vfs.VolumePtr(0)
	realAllocationSize := vfs.VolumePtr(0)
	for i := 0; i < 1e4; i++ {
		allocatedSize, err := vfs.Allocate(vfs.MutableInode{
			Inode:    &inode,
			InodePtr: vfs.VolumePtrToInodePtr(fs.Superblock, inodeObject.VolumePtr),
		}, fs.Volume, fs.Superblock, 10)
		if err != nil {
			t.Fatal(err)
		}

		realAllocationSize += allocatedSize
		expectedAllocationSize += vfs.VolumePtr(fs.Superblock.ClusterSize)
	}

	if realAllocationSize != expectedAllocationSize {
		t.Errorf("allocated %d bytes, expected allocation of %d bytes", realAllocationSize, expectedAllocationSize)
	}
}

func TestShrink(t *testing.T) {
	fs := PrepareFS(1e9, t)
	defer func() {
		_ = fs.Volume.Destroy()
	}()

	inodeObject, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, true)
	if err != nil {
		t.Fatal(err)
	}

	inode := inodeObject.Object.(vfs.Inode)

	// Allocate 1e7 bytes
	allocatedSize, err := vfs.Allocate(vfs.MutableInode{
		Inode:    &inode,
		InodePtr: vfs.VolumePtrToInodePtr(fs.Superblock, inodeObject.VolumePtr),
	}, fs.Volume, fs.Superblock, 1e7)
	if err != nil {
		t.Fatal(err)
	}

	if allocatedSize != vfs.VolumePtr(1e7+384) {
		t.Fatalf("allocated size is incorrect, %d instead of %d", allocatedSize, vfs.VolumePtr(1e7+2432))
	}

	shrinkedSize, err := vfs.Shrink(vfs.MutableInode{
		Inode:    &inode,
		InodePtr: vfs.VolumePtrToInodePtr(fs.Superblock, inodeObject.VolumePtr),
	}, fs.Volume, fs.Superblock, 8000)

	if shrinkedSize != 8000+192 {
		t.Errorf("shrinked size is incorrect, %d instead of %d", shrinkedSize, vfs.VolumePtr(8000+192))
	}
}
