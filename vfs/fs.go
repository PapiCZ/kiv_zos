package vfs

import (
	"math"
	"unsafe"
)

type Filesystem struct {
	Volume          Volume
	Superblock      Superblock
	RootInodePtr    InodePtr
	CurrentInodePtr InodePtr
}

func NewFilesystem(volume Volume, clusterSize int16) (Filesystem, error) {
	volumeSize, err := volume.Size()
	if err != nil {
		return Filesystem{}, err
	}

	metadataSize := VolumePtr(float64(volumeSize) * 0.05) // 5%
	dataSize := VolumePtr(float64(volumeSize) * 0.95)     // 95%

	sb := NewPreparedSuperblock("janopa", "kiv/zos", volumeSize, clusterSize)
	sbSize := VolumePtr(unsafe.Sizeof(sb))

	sb.ClusterCount = ClusterPtr((volumeSize - metadataSize) / VolumePtr(clusterSize))

	clusterBitmapSize := VolumePtr(math.Ceil(float64(dataSize/VolumePtr(clusterSize)) / 8))

	// Count inode bitmap size and total inodes size
	inodeSize := VolumePtr(unsafe.Sizeof(Inode{}))
	totalInodesCount := VolumePtr(float64(metadataSize-sbSize-clusterBitmapSize) / (float64(inodeSize) + 1.0/8)) // Just math
	inodeBitmapSize := NeededMemoryForBitmap(totalInodesCount)

	sb.ClusterBitmapStartAddress = sbSize
	sb.InodeBitmapStartAddress = sb.ClusterBitmapStartAddress + clusterBitmapSize
	sb.InodesStartAddress = sb.InodeBitmapStartAddress + inodeBitmapSize

	sb.DataStartAddress = metadataSize

	return Filesystem{
		Volume:     volume,
		Superblock: sb,
	}, nil
}

func NewFilesystemFromSuperblock(volume Volume, sb Superblock) Filesystem {
	return Filesystem{
		Volume:     volume,
		Superblock: sb,
	}
}

func (f Filesystem) WriteStructureToVolume() error {
	err := f.Volume.Truncate()
	if err != nil {
		return err
	}

	// TODO: Should work without following code
	//err = FreeAllInodes(f.Volume, f.Superblock)
	//if err != nil {
	//	return err
	//}
	//
	//err = FreeAllClusters(f.Volume, f.Superblock)
	//if err != nil {
	//	return err
	//}

	err = f.Volume.WriteStruct(0, f.Superblock)
	if err != nil {
		return err
	}

	return nil
}

func (f Filesystem) ReadCluster(cp ClusterPtr, data interface{}) error {
	err := f.Volume.ReadStruct(ClusterPtrToVolumePtr(f.Superblock, cp), data)
	if err != nil {
		return err
	}

	return nil
}
