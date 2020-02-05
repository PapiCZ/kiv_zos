package vfsapi

import (
	"errors"
	"github.com/PapiCZ/kiv_zos/vfs"
)

func FsCheck(fs vfs.Filesystem) error {
	inodePtrs := make(map[vfs.InodePtr]bool)
	err := readAllInodePtrsRecursively(fs, fs.RootInodePtr, inodePtrs)
	if err != nil {
		return err
	}

	// Check all used inodes
	inodeBytes := make([]byte, fs.Superblock.InodesStartAddress-fs.Superblock.InodeBitmapStartAddress)
	err = fs.Volume.ReadBytes(fs.Superblock.InodeBitmapStartAddress, inodeBytes)
	if err != nil {
		return err
	}

	inodeBitmap := vfs.Bitmap(inodeBytes)
	for i := vfs.VolumePtr(0); i < vfs.VolumePtr(inodeBitmap.Len()); i++ {
		value, err := inodeBitmap.GetBit(i)
		if err != nil {
			return err
		}

		_, ok := inodePtrs[vfs.InodePtr(i)]
		if value == 0 && ok {
			return errors.New("inode is actively used by filesystem, but it should be free")
		} else if value == 1 && !ok {
			return errors.New("found zombie inode that isn't used by filesystem")
		}
	}

	// Check all used data clusters
	clusterBytes := make([]byte, fs.Superblock.InodeBitmapStartAddress-fs.Superblock.ClusterBitmapStartAddress)
	err = fs.Volume.ReadBytes(fs.Superblock.ClusterBitmapStartAddress, clusterBytes)
	if err != nil {
		return err
	}

	clusterBitmap := vfs.Bitmap(clusterBytes)
	for inodePtr, _ := range inodePtrs {
		mutableInode, err := vfs.LoadMutableInode(fs.Volume, fs.Superblock, inodePtr)
		if err != nil {
			return err
		}

		directPtrs, indirect1Ptrs, indirect2Ptrs, err := mutableInode.Inode.GetUsedPtrs(fs.Volume, fs.Superblock)
		if err != nil {
			return err
		}

		// Check direct pointers
		for _, directPtr := range directPtrs {
			value, err := clusterBitmap.GetBit(vfs.VolumePtr(directPtr))
			if err != nil {
				return err
			}

			if value != 1 {
				return errors.New("data cluster should be free but it's used by inode")
			}
		}

		// Check indirect1 pointers
		for k, singlePtrTable := range indirect1Ptrs {
			if k == vfs.Unused {
				break
			}

			value, err := clusterBitmap.GetBit(vfs.VolumePtr(k))
			if err != nil {
				return err
			}

			if value != 1 {
				return errors.New("data cluster should be free but it's used by inode")
			}

			for _, dataPtr := range singlePtrTable {
				value, err := clusterBitmap.GetBit(vfs.VolumePtr(dataPtr))
				if err != nil {
					return err
				}

				if value != 1 {
					return errors.New("data cluster should be free but it's used by inode")
				}
			}
		}

		// Check indirect2 pointers
		for k, doublePtrTable := range indirect2Ptrs {
			if k == vfs.Unused {
				break
			}

			value, err := clusterBitmap.GetBit(vfs.VolumePtr(k))
			if err != nil {
				return err
			}

			if value != 1 {
				return errors.New("data cluster should be free but it's used by inode")
			}

			for k2, singlePtrTable := range doublePtrTable {
				value, err := clusterBitmap.GetBit(vfs.VolumePtr(k2))
				if err != nil {
					return err
				}

				if value != 1 {
					return errors.New("data cluster should be free but it's used by inode")
				}

				for _, dataPtr := range singlePtrTable {
					value, err := clusterBitmap.GetBit(vfs.VolumePtr(dataPtr))
					if err != nil {
						return err
					}

					if value != 1 {
						return errors.New("data cluster should be free but it's used by inode")
					}
				}
			}
		}
	}

	return nil
}

func readAllInodePtrsRecursively(fs vfs.Filesystem, inodePtr vfs.InodePtr, out map[vfs.InodePtr]bool) error {
	parentMutableInode, err := vfs.LoadMutableInode(fs.Volume, fs.Superblock, inodePtr)
	if err != nil {
		return err
	}

	directoryEntries, err := vfs.ReadAllDirectoryEntries(fs.Volume, fs.Superblock, *parentMutableInode.Inode)
	if err != nil {
		return err
	}

	for _, directoryEntry := range directoryEntries {
		_, ok := out[directoryEntry.InodePtr]
		if ok {
			// We already have this inode pointer in map, let's skip it
			continue
		}

		out[directoryEntry.InodePtr] = true

		// Check if directory entry is directory
		mutableInode, err := vfs.LoadMutableInode(fs.Volume, fs.Superblock, directoryEntry.InodePtr)
		if err != nil {
			return err
		}

		if mutableInode.Inode.IsDir() {
			err = readAllInodePtrsRecursively(fs, directoryEntry.InodePtr, out)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
