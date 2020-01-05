package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"testing"
	"unsafe"
)

func TestSuperblockMath(t *testing.T) {
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

	s := fs.Superblock

	metadataSize := vfs.Vptr(1e6 * 0.05)
	superblockSize := vfs.Vptr(unsafe.Sizeof(vfs.Superblock{}))

	if s.BitmapStartAddress != superblockSize {
		t.Errorf("BitmapStartAddress value is not correct! %d, should be %d instead.", s.BitmapStartAddress, superblockSize)
	}
	if s.InodeStartAddress != s.BitmapStartAddress+232 {
		t.Errorf("InodeStartAddress value is not correct! %d, should be %d instead.", s.InodeStartAddress, s.BitmapStartAddress+232)
	}
	if s.DataStartAddress != metadataSize {
		t.Errorf("DataStartAddress value is not correct! %d, should be %d instead.", s.DataStartAddress, metadataSize)
	}
	if s.ClusterCount != (1e6-metadataSize)/vfs.Vptr(512) {
		t.Errorf("ClusterCount value is not correct! %d, should be %d instead.", s.ClusterCount, (1e6-metadataSize)/vfs.Vptr(512))
	}
}
