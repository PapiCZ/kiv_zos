package tests

import (
	"bytes"
	"github.com/PapiCZ/kiv_zos/vfs"
	"testing"
)

func PrepareInode(fsSize vfs.VolumePtr, allocationSize vfs.VolumePtr, t *testing.T) (vfs.Filesystem, vfs.Inode, vfs.InodePtr) {
	// Create volume
	path := tempFileName("", "")
	err := vfs.PrepareVolumeFile(path, fsSize)

	volume, err := vfs.NewVolume(path)
	if err != nil {
		t.Fatal(err)
	}

	// Create filesystem
	fs, err := vfs.NewFilesystem(volume, 4096)
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

	vo, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, false)
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

	// Allocate
	inodeObject, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, false)
	if err != nil {
		t.Fatal(err)
	}

	inode := inodeObject.Object.(vfs.Inode)
	_, err = vfs.Allocate(vfs.MutableInode{
		Inode:    &inode,
		InodePtr: vfs.VolumePtrToInodePtr(fs.Superblock, inodeObject.VolumePtr),
	}, fs.Volume, fs.Superblock, allocationSize)
	if err != nil {
		t.Fatal(err)
	}

	return vfs.Filesystem{
		Volume:     volume,
		Superblock: s,
	}, inode, vfs.VolumePtrToInodePtr(fs.Superblock, inodeObject.VolumePtr)
}

func TestClusterAddressResolution(t *testing.T) {
	fs, inode, _ := PrepareInode(1e9, 1e8, t)
	defer func() {
		_ = fs.Volume.Destroy()
	}()

	// Test direct pointers
	for i := vfs.ClusterPtr(0); i < 5; i++ {
		clusterPtr, err := inode.ResolveDataClusterAddress(fs.Volume, fs.Superblock, i)
		if err != nil {
			t.Fatal(err)
		}
		if clusterPtr != i {
			t.Errorf("direct address is not correct, %d instead of %d", clusterPtr, i)
		}
	}

	// Test indirect1 pointers
	if inode.Indirect1 != 5 {
		t.Errorf("indirect1 address is not correct, %d instead of %d", inode.Indirect1, 5)
	}

	for i := vfs.ClusterPtr(5); i < 1029; i++ {
		clusterPtr, err := inode.ResolveDataClusterAddress(fs.Volume, fs.Superblock, i)
		if err != nil {
			t.Fatal(err)
		}
		if clusterPtr != i+1 {
			t.Errorf("address stored in indirect1 is not correct, %d instead of %d", clusterPtr, i+1)
		}
	}

	// Test indirect2 pointers
	if inode.Indirect2 != 1030 {
		t.Errorf("indirect2 address is not correct, %d instead of %d", inode.Indirect2, 517)
	}

	for i := vfs.ClusterPtr(1054); i < 24415; i++ {
		clusterPtr, err := inode.ResolveDataClusterAddress(fs.Volume, fs.Superblock, i)
		if err != nil {
			t.Fatal(err)
		}
		if clusterPtr != i+2 {
			t.Errorf("address stored in indirect2 is not correct, %d instead of %d", clusterPtr, i+2)
		}
	}
}

func TestAppendData(t *testing.T) {
	fs, inode, ptr := PrepareInode(1e8, 1e7, t)
	defer func() {
		_ = fs.Volume.Destroy()
	}()

	mutableInode := vfs.MutableInode{
		Inode:    &inode,
		InodePtr: ptr,
	}

	// Generate data
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	data := make([]byte, 1e7)
	for i := 0; i < len(data); i++ {
		data[i] = charset[i%len(charset)]
	}

	n, err := mutableInode.AppendData(fs.Volume, fs.Superblock, data[:1_000_000])
	if err != nil {
		t.Fatal(err)
	}

	if n != 1e6 {
		t.Error("not all data was written")
	}

	n, err = mutableInode.AppendData(fs.Volume, fs.Superblock, data[1_000_000:])
	if err != nil {
		t.Fatal(err)
	}

	if n != 1e7-1e6 {
		t.Error("not all data was written")
	}

	// Compare written data
	clusterData := make([]byte, fs.Superblock.ClusterSize)
	i := vfs.ClusterPtr(0)
	for {
		clusterPtr, err := inode.ResolveDataClusterAddress(fs.Volume, fs.Superblock, i)
		if err != nil {
			t.Fatal(err)
		}

		err = fs.Volume.ReadBytes(vfs.ClusterPtrToVolumePtr(fs.Superblock, clusterPtr), clusterData)
		if err != nil {
			t.Fatal(err)
		}

		start := i * vfs.ClusterPtr(fs.Superblock.ClusterSize)
		end := (i + 1) * vfs.ClusterPtr(fs.Superblock.ClusterSize)

		if end > vfs.ClusterPtr(len(data)) {
			end = vfs.ClusterPtr(len(data))
		}

		diff := bytes.Compare(clusterData[:end-start], data[start:end])
		if diff != 0 {
			t.Fatalf("read and written data are not equal, failure in cluster %d", clusterPtr)
		}

		i++

		if end == vfs.ClusterPtr(len(data)) {
			break
		}
	}
}

func TestAppendDataReallocation(t *testing.T) {
	fs, inode, ptr := PrepareInode(1e8, 10, t)
	defer func() {
		_ = fs.Volume.Destroy()
	}()

	mutableInode := vfs.MutableInode{
		Inode:    &inode,
		InodePtr: ptr,
	}

	// Generate data
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	data := make([]byte, 1e7)
	for i := 0; i < len(data); i++ {
		data[i] = charset[i%len(charset)]
	}

	for i := 0; i < len(data)/100; i++ {
		_, err := mutableInode.AppendData(fs.Volume, fs.Superblock, data[i*100:(i+1)*100])
		if err != nil {
			t.Fatal(err)
		}
	}

	// Compare written data
	clusterData := make([]byte, fs.Superblock.ClusterSize)
	i := vfs.ClusterPtr(0)
	for {
		clusterPtr, err := inode.ResolveDataClusterAddress(fs.Volume, fs.Superblock, i)
		if err != nil {
			t.Fatal(err)
		}

		err = fs.Volume.ReadBytes(vfs.ClusterPtrToVolumePtr(fs.Superblock, clusterPtr), clusterData)
		if err != nil {
			t.Fatal(err)
		}

		start := i * vfs.ClusterPtr(fs.Superblock.ClusterSize)
		end := (i + 1) * vfs.ClusterPtr(fs.Superblock.ClusterSize)

		if end > vfs.ClusterPtr(len(data)) {
			end = vfs.ClusterPtr(len(data))
		}

		diff := bytes.Compare(clusterData[:end-start], data[start:end])
		if diff != 0 {
			t.Fatalf("read and written data are not equal, failure in cluster %d", clusterPtr)
		}

		i++

		if end == vfs.ClusterPtr(len(data)) {
			break
		}
	}
}
