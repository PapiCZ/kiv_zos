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
	Name     [DirectoryEntryNameLength]byte
	InodePtr InodePtr
}

func NewDirectoryEntry(name string, inodePtr InodePtr) DirectoryEntry {
	return DirectoryEntry{
		Name:     StringNameToBytes(name),
		InodePtr: inodePtr,
	}
}

func InitRootDirectory(fs *Filesystem, mutableInode *MutableInode) error {
	mutableInode.Inode.Type = InodeRootInodeType
	err := mutableInode.Save(fs.Volume, fs.Superblock)
	if err != nil {
		return err
	}

	err = AppendDirectoryEntries(fs.Volume, fs.Superblock, *mutableInode,
		NewDirectoryEntry(".", mutableInode.InodePtr),
		NewDirectoryEntry("..", mutableInode.InodePtr),
	)
	if err != nil {
		return err
	}

	err = mutableInode.Save(fs.Volume, fs.Superblock)
	if err != nil {
		return err
	}

	fs.RootInodePtr = mutableInode.InodePtr
	fs.CurrentInodePtr = mutableInode.InodePtr

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
	},
		NewDirectoryEntry(".", VolumePtrToInodePtr(sb, inodeObj.VolumePtr)),
		NewDirectoryEntry("..", parentInodePtr),
	)
	if err != nil {
		return inode, err
	}

	err = AppendDirectoryEntries(
		volume,
		sb,
		parent,
		NewDirectoryEntry(name, VolumePtrToInodePtr(sb, inodeObj.VolumePtr)),
	)
	if err != nil {
		return inode, err
	}

	return inode, nil
}

func AppendDirectoryEntries(volume ReadWriteVolume, sb Superblock, inode MutableInode, directoryEntries ...DirectoryEntry) error {
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

func FindDirectoryEntryByInodePtr(volume ReadWriteVolume, sb Superblock, inode Inode, inodePtr InodePtr) (DEPtr, DirectoryEntry, error) {
	directoryEntries, err := ReadAllDirectoryEntries(volume, sb, inode)
	if err != nil {
		return 0, DirectoryEntry{}, err
	}

	for i, directoryEntry := range directoryEntries {
		if directoryEntry.InodePtr == inodePtr {
			return DEPtr(i), directoryEntry, nil
		}
	}

	return 0, DirectoryEntry{}, DirectoryEntryNotFound{}
}

func RemoveDirectoryEntry(volume ReadWriteVolume, sb Superblock, mutableInode MutableInode, name string) (DirectoryEntry, error) {
	directoryEntries, err := ReadAllDirectoryEntries(volume, sb, *mutableInode.Inode)
	if err != nil {
		return DirectoryEntry{}, err
	}

	var foundDirectoryEntry DirectoryEntry
	for i := 0; i < len(directoryEntries); i++ {
		if directoryEntries[i].Name == StringNameToBytes(name) {
			foundDirectoryEntry = directoryEntries[i]

			// Delete matching directory entry
			directoryEntries = append(directoryEntries[:i], directoryEntries[i+1:]...)
		}
	}

	err = SaveDirectoryEntries(volume, sb, mutableInode, directoryEntries)
	if err != nil {
		return foundDirectoryEntry, err
	}

	return foundDirectoryEntry, nil
}

func SaveDirectoryEntries(volume ReadWriteVolume, sb Superblock, mutableInode MutableInode, directoryEntries []DirectoryEntry) error {
	_, err := Shrink(mutableInode, volume, sb, 0)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.LittleEndian, directoryEntries)
	if err != nil {
		return err
	}

	_, err = mutableInode.AppendData(volume, sb, buf.Bytes())
	if err != nil {
		return err
	}

	err = mutableInode.Save(volume, sb)
	if err != nil {
		return err
	}

	return nil
}

func StringNameToBytes(name string) [DirectoryEntryNameLength]byte {
	var nameBytes [DirectoryEntryNameLength]byte
	copy(nameBytes[:], name)
	return nameBytes
}
