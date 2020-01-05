package tests

import (
	"fmt"
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

	address, inode, err := vfs.FindFreeInode(fs.Volume, fs.Superblock)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(address, inode)
}