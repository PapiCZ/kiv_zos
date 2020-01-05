package vfs

type Superblock struct {
	Signature          [9]byte
	VolumeDescriptor   [251]byte
	DiskSize           Vptr
	ClusterSize        int16
	ClusterCount       Vptr
	BitmapStartAddress Vptr
	InodeStartAddress  Vptr
	DataStartAddress   Vptr
}

func NewPreparedSuperblock(signature, volumeDescriptor string, diskSize Vptr, clusterSize int16) Superblock {
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
