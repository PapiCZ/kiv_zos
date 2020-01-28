package vfs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unsafe"
)

type DEPtr int32

type DirectoryEntryNotFound struct {
	Name string
}

func (d DirectoryEntryNotFound) Error() string {
	return fmt.Sprintf("directory entry with name %s was not found", d.Name)
}

const DirectoryEntryNameLength = 12

type DirectoryEntry struct {
	Name  [DirectoryEntryNameLength]byte
	Inode InodePtr
}

func InitRootDirectory(volume ReadWriteVolume, sb Superblock, inodePtr InodePtr, inode MutableInode) error {
	inode.Inode.Type = InodeDirectoryType

	err := AppendDirectoryEntries(volume, sb, inode, []DirectoryEntry{
		{StringNameToBytes("."), inodePtr},
		{StringNameToBytes(".."), inodePtr},
	})
	if err != nil {
		return err
	}

	err = inode.Save(volume, sb)
	if err != nil {
		return err
	}

	return nil
}

func CreateNewDirectory(volume ReadWriteVolume, sb Superblock, parentInodePtr InodePtr, parent MutableInode, name string) (Inode, error) {
	inodeObj, err := FindFreeInode(volume, sb, true)
	if err != nil {
		return Inode{}, err
	}
	inode := inodeObj.Object.(Inode)

	err = AppendDirectoryEntries(volume, sb, MutableInode{
		Inode:    &inode,
		InodePtr: VolumePtrToInodePtr(sb, inodeObj.VolumePtr),
	}, []DirectoryEntry{
		{StringNameToBytes("."), VolumePtrToInodePtr(sb, inodeObj.VolumePtr)},
		{StringNameToBytes(".."), parentInodePtr},
	})
	if err != nil {
		return inode, err
	}

	err = AppendDirectoryEntries(volume, sb, parent, []DirectoryEntry{
		{StringNameToBytes(name), VolumePtrToInodePtr(sb, inodeObj.VolumePtr)},
	})
	if err != nil {
		return inode, err
	}

	return inode, nil
}

func AppendDirectoryEntries(volume ReadWriteVolume, sb Superblock, inode MutableInode, directoryEntries []DirectoryEntry) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, directoryEntries)
	if err != nil {
		return err
	}
	_, err = inode.AppendData(volume, sb, buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func ReadAllDirectoryEntries(volume ReadWriteVolume, sb Superblock, inode Inode) ([]DirectoryEntry, error) {
	directoryEntryBytes := make([]byte, inode.Size)
	_, err := inode.ReadData(volume, sb, 0, directoryEntryBytes)
	if err != nil {
		return nil, err
	}
	directoryEntries := make([]DirectoryEntry, len(directoryEntryBytes)/int(unsafe.Sizeof(DirectoryEntry{})))
	reader := bytes.NewReader(directoryEntryBytes)
	err = binary.Read(reader, binary.LittleEndian, &directoryEntries)
	if err != nil {
		return nil, err
	}
	return directoryEntries, nil
}

func FindDirectoryEntryByName(volume ReadWriteVolume, sb Superblock, inode Inode, name string) (DEPtr, DirectoryEntry, error) {
	directoryEntries, err := ReadAllDirectoryEntries(volume, sb, inode)
	if err != nil {
		return 0, DirectoryEntry{}, err
	}

	nameBytes := StringNameToBytes(name)
	for i, directoryEntry := range directoryEntries {
		if directoryEntry.Name == nameBytes {
			return DEPtr(i), directoryEntry, nil
		}
	}

	return 0, DirectoryEntry{}, DirectoryEntryNotFound{name}
}

//func RemoveDirectoryEntry(volume ReadWriteVolume, sb Superblock, inode Inode, name string) error {
//}

func StringNameToBytes(name string) [DirectoryEntryNameLength]byte {
	var nameBytes [DirectoryEntryNameLength]byte
	copy(nameBytes[:], name)
	return nameBytes
}
