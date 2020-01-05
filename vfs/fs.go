package vfs

import (
	"unsafe"
)

type Filesystem struct {
	Volume     Volume
	Superblock Superblock
}

func NewFilesystem(volume Volume, clusterSize int16) (Filesystem, error) {
	volumeSize, err := volume.Size()
	if err != nil {
		return Filesystem{}, err
	}

	metadataSize := Vptr(float64(volumeSize) * 0.05) // 5%
	dataSize := volumeSize - metadataSize

	s := NewPreparedSuperblock("janopa", "kiv/zos", volumeSize, clusterSize)
	superblockSize := Vptr(unsafe.Sizeof(s))
	inodeSize := Vptr(unsafe.Sizeof(Inode{}))

	s.BitmapStartAddress = superblockSize
	s.ClusterCount = dataSize / Vptr(clusterSize)
	s.InodeStartAddress = s.BitmapStartAddress + NeededMemoryForBitmap(s.ClusterCount)
	s.DataStartAddress = s.InodeStartAddress + inodeSize * (metadataSize - s.InodeStartAddress /* ??? - 1 ??? */)

	return Filesystem{
		Volume:     volume,
		Superblock: s,
	}, nil
}

func (f Filesystem) WriteStructureToVolume() error {
	err := f.Volume.Truncate()
	if err != nil {
		return err
	}

	err = f.Volume.WriteStruct(0, f.Superblock)
	if err != nil {
		return err
	}

	return nil
}
