package vfs

type Superblock struct {
	signature          [9]byte
	volumeDescriptor   [251]byte
	diskSize           Address
	clusterSize        int16
	clusterCount       int64
	bitmapStartAddress Address
	inodeStartAddress  Address
	dataStartAddress   Address
}

