package vfs

import (
	"unsafe"
)

type Filesystem struct {
	Volume     Volume
	Superblock Superblock
}

func NewFilesystem(volume Volume, clusterSize int32) (Filesystem, error) {
	volumeSize, err := volume.Size()
	if err != nil {
		return Filesystem{}, err
	}

	metadataSize := int32(float64(volumeSize) * 0.05) // 5%
	dataSize := int32(volumeSize - volumeSize)

	s := NewPreparedSuperblock("janopa", "kiv/zos", int32(volumeSize), int16(clusterSize))
	superblockSize := int32(unsafe.Sizeof(s))
	inodeSize := int32(unsafe.Sizeof(Inode{}))

	s.BitmapStartAddress = superblockSize
	s.ClusterCount = dataSize / clusterSize
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
