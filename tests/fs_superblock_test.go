package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"testing"
	"unsafe"
)

func TestSuperblockMath(t *testing.T) {
	// Create volume
	path := tempFileName("", "")
	err := vfs.PrepareVolumeFile(path, 1e4) // 10 000B

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

	metadataSize := vfs.VolumePtr(1e4 * 0.05)

	superblockSize := vfs.VolumePtr(unsafe.Sizeof(vfs.Superblock{}))

	clusterBitmapSize := vfs.VolumePtr(3)
	inodeBitmapSize := vfs.VolumePtr(1)

	if s.ClusterBitmapStartAddress != superblockSize {
		t.Errorf("ClusterBitmapStartAddress value is not correct! %d, should be %d instead.", s.ClusterBitmapStartAddress, superblockSize)
	}
	if s.InodeBitmapStartAddress != s.ClusterBitmapStartAddress + clusterBitmapSize {
		t.Errorf("InodeBitmapStartAddress value is not correct! %d, should be %d instead.", s.InodeBitmapStartAddress, s.ClusterBitmapStartAddress + clusterBitmapSize)
	}
	if s.InodesStartAddress != s.InodeBitmapStartAddress + inodeBitmapSize {
		t.Errorf("InodesStartAddress value is not correct! %d, should be %d instead.", s.InodesStartAddress, s.InodeBitmapStartAddress + inodeBitmapSize)
	}
	if s.DataStartAddress != metadataSize {
		t.Errorf("DataStartAddress value is not correct! %d, should be %d instead.", s.DataStartAddress, metadataSize)
	}
	if s.ClusterCount != (1e4-metadataSize)/vfs.VolumePtr(512) {
		t.Errorf("ClusterCount value is not correct! %d, should be %d instead.", s.ClusterCount, (1e6-metadataSize)/vfs.VolumePtr(512))
	}
}
