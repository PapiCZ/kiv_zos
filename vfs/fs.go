package vfs

import (
	"unsafe"
)

type Filesystem struct {
	Volume     Volume
	Superblock Superblock
	Bitmap     Bitmap
}

func NewFilesystem(volume Volume, clusterSize int16) (Filesystem, error) {
	volumeSize, err := volume.Size()
	if err != nil {
		return Filesystem{}, err
	}

	metadataSize := Vptr(float64(volumeSize) * 0.05) // 5%

	s := NewPreparedSuperblock("janopa", "kiv/zos", volumeSize, clusterSize)
	superblockSize := Vptr(unsafe.Sizeof(s))

	s.BitmapStartAddress = superblockSize
	s.ClusterCount = (volumeSize - metadataSize) / Vptr(clusterSize)
	s.InodeStartAddress = s.BitmapStartAddress + NeededMemoryForBitmap(s.ClusterCount)
	s.DataStartAddress = metadataSize

	bitmap := NewBitmap(s.ClusterCount)

	return Filesystem{
		Volume:     volume,
		Superblock: s,
		Bitmap:     bitmap,
	}, nil
}

func (f Filesystem) WriteStructureToVolume() error {
	err := f.Volume.Truncate()
	if err != nil {
		return err
	}

	err = FreeAllInodes(f.Volume, f.Superblock)
	if err != nil {
		return err
	}

	err = f.Volume.WriteStruct(0, f.Superblock)
	if err != nil {
		return err
	}

	return nil
}
