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

func TestRenameDirectory(t *testing.T) {
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
	err = vfsapi.Rename(fs, "bardir2", "barfoodir4")
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

	if len(files) != 5 {
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

	if files[4].Name() != "barfoodir4" {
		t.Errorf("bad file name, %s instead of %s", files[4].Name(), "barfoodir4")
	}

	if !files[4].IsDir() {
		t.Error("file should be directory")
	}
}

func TestRenameNestedDirectory(t *testing.T) {
	fs := PrepareFSForApi(1e7, t)
	err := vfsapi.Mkdir(fs, "foodir1")
	if err != nil {
		t.Fatal(err)
	}

	err = vfsapi.Mkdir(fs, "foodir1/bar1")
	if err != nil {
		t.Fatal(err)
	}

	err = vfsapi.Mkdir(fs, "foodir2")
	if err != nil {
		t.Fatal(err)
	}
	err = vfsapi.Mkdir(fs, "foodir2/bar2")
	if err != nil {
		t.Fatal(err)
	}

	// Rename directory
	err = vfsapi.Rename(fs, "foodir2/bar2", "foodir1")
	if err != nil {
		t.Fatal(err)
	}

	// List root directory
	rootFile, err := vfsapi.Open(fs, "/")
	if err != nil {
		t.Fatal(err)
	}

	rootFiles, err := rootFile.ReadDir()
	if err != nil {
		t.Fatal(err)
	}

	if len(rootFiles) != 4 {
		t.Error("expected 4 directories in root directory")
	}

	for k, v := range []string{".", "..", "foodir1", "foodir2"} {
		if rootFiles[k].Name() != v {
			t.Errorf("bad file name, %s instead of %s", rootFiles[k].Name(), v)
		}

		if !rootFiles[k].IsDir() {
			t.Error("file should be directory")
		}
	}

	// List foodir1 directory
	fooDir1File, err := vfsapi.Open(fs, "foodir1")
	if err != nil {
		t.Fatal(err)
	}

	fooDir1Files, err := fooDir1File.ReadDir()
	if err != nil {
		t.Fatal(err)
	}

	if len(fooDir1Files) != 4 {
		t.Error("expected 4 directories in foodir1")
	}

	for k, v := range []string{".", "..", "bar1", "bar2"} {
		if fooDir1Files[k].Name() != v {
			t.Errorf("bad file name, %s instead of %s", fooDir1Files[k].Name(), v)
		}

		if !fooDir1Files[k].IsDir() {
			t.Error("file should be directory")
		}
	}

	// List foodir2 directory
	fooDir2File, err := vfsapi.Open(fs, "foodir2")
	if err != nil {
		t.Fatal(err)
	}

	fooDir2Files, err := fooDir2File.ReadDir()
	if err != nil {
		t.Fatal(err)
	}

	if len(fooDir2Files) != 2 {
		t.Error("expected 2 directories in foodir2")
	}

	for k, v := range []string{".", ".."} {
		if fooDir2Files[k].Name() != v {
			t.Errorf("bad file name, %s instead of %s", fooDir2Files[k].Name(), v)
		}

		if !fooDir2Files[k].IsDir() {
			t.Error("file should be directory")
		}
	}
}

func TestCreateNewFile(t *testing.T) {
	fs := PrepareFSForApi(1e7, t)

	_, err := vfsapi.Open(fs, "/myfile")
	if err != nil {
		t.Fatal(err)
	}

	rootFile, err := vfsapi.Open(fs, "/")
	if err != nil {
		t.Fatal(err)
	}

	files, err := rootFile.ReadDir()
	if err != nil {
		t.Fatal(err)
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

	if files[2].Name() != "myfile" {
		t.Errorf("bad file name, %s instead of %s", files[2].Name(), "myfile")
	}

	if files[2].IsDir() {
		t.Error("file should not be directory")
	}
}
