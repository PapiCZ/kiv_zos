package tests

import (
	"bytes"
	"github.com/PapiCZ/kiv_zos/vfs"
	"testing"
)

func PrepareInode(fsSize vfs.VolumePtr, allocationSize vfs.VolumePtr, t *testing.T) (vfs.Filesystem, vfs.Inode) {
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

	// Allocate
	inode, _, err := vfs.Allocate(fs.Volume, fs.Superblock, allocationSize)
	if err != nil {
		t.Fatal(err)
	}

	return vfs.Filesystem{
		Volume:     volume,
		Superblock: s,
	}, inode
}

func TestClusterAddressResolution(t *testing.T) {
	fs, inode := PrepareInode(1e9, 1e8, t)
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
		if clusterPtr != i+25 {
			t.Errorf("address stored in indirect2 is not correct, %d instead of %d", clusterPtr, i+25)
		}
	}
}

func TestAppendData(t *testing.T) {
	fs, inode := PrepareInode(1e8, 1e7, t)
	defer func() {
		_ = fs.Volume.Destroy()
	}()

	// Generate data
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	data := make([]byte, 1e7)
	for i := 0; i < len(data); i++ {
		data[i] = charset[i%len(charset)]
	}

	n, err := inode.AppendData(fs.Volume, fs.Superblock, data[:1_000_000])
	if err != nil {
		t.Fatal(err)
	}

	if n != 1e6 {
		t.Error("not all data was written")
	}

	n, err = inode.AppendData(fs.Volume, fs.Superblock, data[1_000_000:])
	if err != nil {
		t.Fatal(err)
	}

	if n != 1e7 - 1e6 {
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
