package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"testing"
	"unsafe"
)

func PrepareFS(size vfs.VolumePtr, t *testing.T) vfs.Filesystem {
	// Create volume
	path := tempFileName("", "")
	err := vfs.PrepareVolumeFile(path, size)

	volume, err := vfs.NewVolume(path)
	if err != nil {
		t.Fatal(err)
	}

	// Create filesystem
	fs, err := vfs.NewFilesystem(volume, 512)
	if err != nil {
		t.Fatal(err)
	}

	err = fs.WriteStructureToVolume()
	if err != nil {
		t.Fatal(err)
	}

	// Check written superblock
	s := vfs.Superblock{}
	err = volume.ReadStruct(0, &s)
	if err != nil {
		t.Fatal(err)
	}

	vo, err := vfs.FindFreeInode(fs.Volume, fs.Superblock)
	if err != nil {
		t.Fatal(err)
	}

	if vo.VolumePtr != s.InodesStartAddress {
		t.Errorf("incorrect inode address: %d instead of %d", vo.VolumePtr, s.InodesStartAddress)
	}

	result, err := vfs.IsInodeFree(vo.Volume, s, vfs.VolumePtrToInodePtr(s, vo.VolumePtr))
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Errorf("inode is not free")
	}

	return vfs.Filesystem{
		Volume:     volume,
		Superblock: s,
	}
}

func FindFreeInode(fs vfs.Filesystem, t *testing.T) vfs.VolumeObject {
	vo, err := vfs.FindFreeInode(fs.Volume, fs.Superblock)
	if err != nil {
		t.Fatal(err)
	}
	return vo
}

func TestAllocateDirect(t *testing.T) {
	fs := PrepareFS(1e6, t)
	defer func() {
		_ = fs.Volume.Destroy()
	}()

	inodeObject := FindFreeInode(fs, t)
	inode := inodeObject.Object.(vfs.Inode)

	allocatedSize, err := vfs.AllocateDirect(&inode, fs.Volume, fs.Superblock, 1500)
	if err != nil {
		t.Fatal(err)
	}

	if allocatedSize != 3*vfs.VolumePtr(fs.Superblock.ClusterSize) {
		t.Errorf("allocated incorrect size, %d instead of %d", allocatedSize, 3*fs.Superblock.ClusterSize)
	}

	if inode.Direct1 != vfs.ClusterPtr(0) {
		t.Errorf("invalid cluster in direct1, %d instead of %d", inode.Direct1, vfs.ClusterPtr(0))
	}

	if inode.Direct2 != vfs.ClusterPtr(1) {
		t.Errorf("invalid cluster in direct2, %d instead of %d", inode.Direct2, vfs.ClusterPtr(1))
	}

	if inode.Direct3 != vfs.ClusterPtr(2) {
		t.Errorf("invalid cluster in direct3, %d instead of %d", inode.Direct3, vfs.ClusterPtr(2))
	}

	if inode.Direct4 != vfs.ClusterPtr(0) {
		t.Errorf("invalid cluster in direct4, %d instead of %d", inode.Direct4, vfs.ClusterPtr(0))
	}

	if inode.Direct5 != vfs.ClusterPtr(0) {
		t.Errorf("invalid cluster in direct5, %d instead of %d", inode.Direct5, vfs.ClusterPtr(0))
	}
}

func TestAllocateIndirect1(t *testing.T) {
	fs := PrepareFS(1e6, t)
	defer func() {
		_ = fs.Volume.Destroy()
	}()

	inodeObject := FindFreeInode(fs, t)
	inode := inodeObject.Object.(vfs.Inode)

	allocatedSize, err := vfs.AllocateIndirect1(&inode, fs.Volume, fs.Superblock, 15000)
	if err != nil {
		t.Fatal(err)
	}

	if allocatedSize != 30*vfs.VolumePtr(fs.Superblock.ClusterSize) {
		t.Errorf("allocated incorrect size, %d instead of %d", allocatedSize, 30*fs.Superblock.ClusterSize)
	}

	// Verify data block with pointers
	var vp vfs.VolumePtr
	ptrs := make([]vfs.ClusterPtr, int(fs.Superblock.ClusterSize)/int(unsafe.Sizeof(vp)))
	err = fs.ReadCluster(inode.Indirect1, ptrs)
	if err != nil {
		t.Fatal(err)
	}

	// Verify indirect1 cluster pointer
	if inode.Indirect1 != 0 {
		t.Errorf("incorrect indirect1 pointer, %d instead of %d", inode.Indirect1, 0)
	}

	// Verify used pointers
	for i := 0; i < 30; i++ {
		if int(ptrs[i]) != i+1 {
			t.Errorf("incorrect cluster pointer, %d instead of %d", ptrs[i], i)
		}
	}

	// Verify unused pointers
	for i := 30; i < len(ptrs); i++ {
		if ptrs[i] != 0 {
			t.Errorf("incorrect cluster pointer, %d instead of %d", ptrs[i], 0)
		}
	}
}

func TestAllocateIndirect2(t *testing.T) {
	fs := PrepareFS(1e9, t)
	defer func() {
		_ = fs.Volume.Destroy()
	}()

	inodeObject := FindFreeInode(fs, t)
	inode := inodeObject.Object.(vfs.Inode)

	_, err := vfs.AllocateIndirect2(&inode, fs.Volume, fs.Superblock, 1e7)
	if err != nil {
		t.Fatal(err)
	}

	//if allocatedSize != 19532*vfs.VolumePtr(fs.Superblock.ClusterSize) {
	//	t.Errorf("allocated incorrect size, %d instead of %d", allocatedSize, 30*fs.Superblock.ClusterSize)
	//}
	//
	//// Verify data block with pointers
	//var vp vfs.VolumePtr
	//ptrs := make([]vfs.ClusterPtr, int(fs.Superblock.ClusterSize)/int(unsafe.Sizeof(vp)))
	//err = fs.ReadCluster(inode.Indirect1, ptrs)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//// Verify indirect1 cluster pointer
	//if inode.Indirect1 != 0 {
	//	t.Errorf("incorrect indirect1 pointer, %d instead of %d", inode.Indirect1, 0)
	//}
	//
	//// Verify used pointers
	//for i := 0; i < 30; i++ {
	//	if int(ptrs[i]) != i+1 {
	//		t.Errorf("incorrect cluster pointer, %d instead of %d", ptrs[i], i)
	//	}
	//}
	//
	//// Verify unused pointers
	//for i := 30; i < len(ptrs); i++ {
	//	if ptrs[i] != 0 {
	//		t.Errorf("incorrect cluster pointer, %d instead of %d", ptrs[i], 0)
	//	}
	//}
}
