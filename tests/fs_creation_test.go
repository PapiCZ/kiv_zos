package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"reflect"
	"testing"
)

func TestFilesystemCreation(t *testing.T) {
	// Create volume
	path := tempFileName("", "")
	err := vfs.PrepareVolumeFile(path, 1e6) // 1 000 000B

	volume, err := vfs.NewVolume(path)
	if err != nil {
		t.Fatal(err)
	}

	// Create filesystem
	fs, err := vfs.NewFilesystem(volume, 512)
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

	if !reflect.DeepEqual(s, fs.Superblock) {
		t.Fatal("read and written data are not equal")
	}
}

func TestFreeInode(t *testing.T) {
	// Create volume
	path := tempFileName("", "")
	err := vfs.PrepareVolumeFile(path, 1e6) // 1 000 000B

	volume, err := vfs.NewVolume(path)
	if err != nil {
		t.Fatal(err)
	}

	// Create filesystem
	fs, err := vfs.NewFilesystem(volume, 512)
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

	vo, err := vfs.FindFreeInode(fs.Volume, fs.Superblock)
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
}

func TestFreeInodeWithOccupiedInode(t *testing.T) {
	// Create volume
	path := tempFileName("", "")
	err := vfs.PrepareVolumeFile(path, 1e6) // 1 000 000B

	volume, err := vfs.NewVolume(path)
	if err != nil {
		t.Fatal(err)
	}

	// Create filesystem
	fs, err := vfs.NewFilesystem(volume, 512)
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

	vo, err := vfs.FindFreeInode(fs.Volume, fs.Superblock)
	if err != nil {
		t.Fatal(err)
	}

	err = vfs.OccupyInode(vo.Volume, s, vfs.VolumePtrToInodePtr(s, vo.VolumePtr))
	if err != nil {
		t.Fatal(err)
	}

	vo2, err := vfs.FindFreeInode(fs.Volume, fs.Superblock)
	if err != nil {
		t.Fatal(err)
	}

	if vfs.VolumePtrToInodePtr(s, vo2.VolumePtr) != 1 {
		t.Errorf("incorrect inode address: %d instead of %d", vfs.VolumePtrToInodePtr(s, vo2.VolumePtr), 1)
	}

	isFree, err := vfs.IsInodeFree(vo2.Volume, s, vfs.VolumePtrToInodePtr(s, vo2.VolumePtr))
	if err != nil {
		t.Fatal(err)
	}
	if !isFree {
		t.Errorf("inode is not free")
	}
}

