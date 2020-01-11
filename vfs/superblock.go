package vfs

type Superblock struct {
	Signature                 [9]byte
	VolumeDescriptor          [251]byte
	DiskSize                  VolumePtr
	ClusterSize               int16
	ClusterCount              VolumePtr
	ClusterBitmapStartAddress VolumePtr
	InodeBitmapStartAddress   VolumePtr
	InodesStartAddress        VolumePtr
	DataStartAddress          VolumePtr
}

func NewPreparedSuperblock(signature, volumeDescriptor string, diskSize VolumePtr, clusterSize int16) Superblock {
	var signatureBytes [9]byte
	copy(signatureBytes[:], signature)

	var volumeDescriptorBytes [251]byte
	copy(volumeDescriptorBytes[:], volumeDescriptor)

	return Superblock{
		Signature:                 signatureBytes,
		VolumeDescriptor:          volumeDescriptorBytes,
		DiskSize:                  diskSize,
		ClusterSize:               clusterSize,
		ClusterCount:              0,
		ClusterBitmapStartAddress: 0,
		InodesStartAddress:        0,
		DataStartAddress:          0,
	}
}
