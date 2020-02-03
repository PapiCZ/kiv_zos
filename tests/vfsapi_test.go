package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"github.com/PapiCZ/kiv_zos/vfsapi"
	"testing"
)

func PrepareFSForApi(size vfs.VolumePtr, t *testing.T) vfs.Filesystem {
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

	rootInodeObj, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, true)
	if err != nil {
		t.Fatal(err)
	}
	rootInode := rootInodeObj.Object.(vfs.Inode)

	err = vfs.InitRootDirectory(&fs, &vfs.MutableInode{
		Inode:    &rootInode,
		InodePtr: vfs.VolumePtrToInodePtr(fs.Superblock, rootInodeObj.VolumePtr),
	})
	if err != nil {
		t.Fatal(err)
	}

	return fs
}


func TestListDirectories(t *testing.T) {
	fs := PrepareFSForApi(1e7, t)
	file, err := vfsapi.Open(fs, "/")
	if err != nil {
		t.Fatal(err)
	}

	files, err := file.ReadDir()
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 2 {
		t.Error("expected 2 directories")
	}

	if files[0].Name() != "." {
		t.Errorf("bad file name, %s instead of %s", files[0].Name(), ".")
	}

	if !files[0].IsDir() {
		t.Error("file should be directory")
	}

	if files[1].Name() != ".." {
		t.Errorf("bad file name, %s instead of %s", files[1].Name(), "..")
	}

	if !files[1].IsDir() {
		t.Error("file should be directory")
	}
}

func TestListDirectoriesWithCreatedDirectory(t *testing.T) {
	fs := PrepareFSForApi(1e7, t)
	err := vfsapi.Mkdir(fs, "foodir1")
	if err != nil {
		t.Fatal(err)
	}

	err = vfsapi.Mkdir(fs, "bardir2")
	if err != nil {
		t.Fatal(err)
	}

	file, err := vfsapi.Open(fs, "/")
	if err != nil {
		t.Fatal(err)
	}

	files, err := file.ReadDir()
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 4 {
		t.Error("expected 4 directories")
	}

	if files[0].Name() != "." {
		t.Errorf("bad file name, %s instead of %s", files[0].Name(), ".")
	}

	if !files[0].IsDir() {
		t.Error("file should be directory")
	}

	if files[1].Name() != ".." {
		t.Errorf("bad file name, %s instead of %s", files[1].Name(), "..")
	}

	if !files[1].IsDir() {
		t.Error("file should be directory")
	}

	if files[2].Name() != "foodir1" {
		t.Errorf("bad file name, %s instead of %s", files[2].Name(), "foodir1")
	}

	if !files[2].IsDir() {
		t.Error("file should be directory")
	}

	if files[3].Name() != "bardir2" {
		t.Errorf("bad file name, %s instead of %s", files[3].Name(), "bardir2")
	}

	if !files[3].IsDir() {
		t.Error("file should be directory")
	}
}

func TestCreateNestedDirectory(t *testing.T) {
	fs := PrepareFSForApi(1e7, t)
	err := vfsapi.Mkdir(fs, "foodir1")
	if err != nil {
		t.Fatal(err)
	}

	err = vfsapi.Mkdir(fs, "/foodir1/bardir2")
	if err != nil {
		t.Fatal(err)
	}

	file, err := vfsapi.Open(fs, "/foodir1")
	if err != nil {
		t.Fatal(err)
	}

	files, err := file.ReadDir()
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 3 {
		t.Error("expected 3 directories")
	}

	if files[0].Name() != "." {
		t.Errorf("bad file name, %s instead of %s", files[0].Name(), ".")
	}

	if !files[0].IsDir() {
		t.Error("file should be directory")
	}

	if files[1].Name() != ".." {
		t.Errorf("bad file name, %s instead of %s", files[1].Name(), "..")
	}

	if !files[1].IsDir() {
		t.Error("file should be directory")
	}

	if files[2].Name() != "bardir2" {
		t.Errorf("bad file name, %s instead of %s", files[2].Name(), "bardir2")
	}

	if !files[2].IsDir() {
		t.Error("file should be directory")
	}
}

func TestDeleteEmptyDirectory(t *testing.T) {
	fs := PrepareFSForApi(1e7, t)
	err := vfsapi.Mkdir(fs, "foodir1")
	if err != nil {
		t.Fatal(err)
	}

	err = vfsapi.Mkdir(fs, "bardir2")
	if err != nil {
		t.Fatal(err)
	}

	err = vfsapi.Mkdir(fs, "foobardir3")
	if err != nil {
		t.Fatal(err)
	}

	err = vfsapi.Remove(fs, "bardir2")
	if err != nil {
		t.Fatal(err)
	}

	// List directory
	file, err := vfsapi.Open(fs, "/")
	if err != nil {
		t.Fatal(err)
	}

	files, err := file.ReadDir()
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 4 {
		t.Error("expected 4 directories")
	}

	if files[0].Name() != "." {
		t.Errorf("bad file name, %s instead of %s", files[0].Name(), ".")
	}

	if !files[0].IsDir() {
		t.Error("file should be directory")
	}

	if files[1].Name() != ".." {
		t.Errorf("bad file name, %s instead of %s", files[1].Name(), "..")
	}

	if !files[1].IsDir() {
		t.Error("file should be directory")
	}

	if files[2].Name() != "foodir1" {
		t.Errorf("bad file name, %s instead of %s", files[2].Name(), "foodir1")
	}

	if !files[2].IsDir() {
		t.Error("file should be directory")
	}

	if files[3].Name() != "foobardir3" {
		t.Errorf("bad file name, %s instead of %s", files[3].Name(), "foobardir3")
	}

	if !files[3].IsDir() {
		t.Error("file should be directory")
	}
}

