package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"math"
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

	superblockStructSize := int32(unsafe.Sizeof(vfs.Superblock{}))

	if s.BitmapStartAddress != superblockStructSize {
		t.Errorf("BitmapStartAddress value is not correct! %d, should be %d instead.", s.BitmapStartAddress, superblockStructSize)
	}
	if s.ClusterCount != int32(math.Floor(0.95 * 1e6 / 512)) {
		t.Errorf("ClusterCount value is not correct! %d, should be %d instead.", s.ClusterCount, int32(math.Floor(0.95 * 1e6 / 512)))
	}
	if s.InodeStartAddress != 0 {
		t.Errorf("InodeStartAddress value is not correct! %d, should be %d instead.", s.InodeStartAddress, 0)
	}
	if s.DataStartAddress != 0 {
		t.Errorf("DataStartAddress value is not correct! %d, should be %d instead.", s.DataStartAddress, 0)
	}
}
