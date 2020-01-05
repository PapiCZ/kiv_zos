package vfs

type Superblock struct {
	Signature          [9]byte
	VolumeDescriptor   [251]byte
	DiskSize           int32
	ClusterSize        int16
	ClusterCount       int32
	BitmapStartAddress int32
	InodeStartAddress  int32
	DataStartAddress   int32
}

func NewPreparedSuperblock(signature, volumeDescriptor string, diskSize int32, clusterSize int16) Superblock {
	var signatureBytes [9]byte
	copy(signatureBytes[:], signature)

	var volumeDescriptorBytes [251]byte
	copy(volumeDescriptorBytes[:], volumeDescriptor)


	return Superblock{
		Signature:          signatureBytes,
		VolumeDescriptor:   volumeDescriptorBytes,
		DiskSize:           diskSize,
		ClusterSize:        clusterSize,
		ClusterCount:       0,
		BitmapStartAddress: 0,
		InodeStartAddress:  0,
		DataStartAddress:   0,
	}
}
