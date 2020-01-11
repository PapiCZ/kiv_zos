package vfs

import (
	"encoding/binary"
	"errors"
	"os"
)

type VolumePtr int64
type ClusterPtr int32
type InodePtr int32

type Volume struct {
	file       *os.File
	endianness binary.ByteOrder
}

func PrepareVolumeFile(path string, size int64) error {
	f, err := os.Create(path)

	defer func() {
		_ = f.Close()
	}()

	if err != nil {
		return err
	}

	err = f.Truncate(size)
	if err != nil {
		return err
	}

	return nil
}

func NewVolume(path string) (Volume, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return Volume{}, err
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return Volume{}, err
	}

	return Volume{
		file:       f,
		endianness: binary.LittleEndian,
	}, nil
}

func (v *Volume) goToAddress(volumePtr VolumePtr) error {
	_, err := v.file.Seek(int64(volumePtr), 0)
	if err != nil {
		return err
	}

	return nil
}

func (v Volume) WriteStruct(volumePtr VolumePtr, data interface{}) error {
	err := v.goToAddress(volumePtr)
	if err != nil {
		return err
	}
	err = binary.Write(v.file, v.endianness, data)
	if err != nil {
		return err
	}

	return nil
}

func (v Volume) WriteByte(volumePtr VolumePtr, data byte) error {
	err := v.goToAddress(volumePtr)
	if err != nil {
		return err
	}

	_, err = v.file.Write([]byte{data})
	if err != nil {
		return err
	}

	return nil
}

func (v Volume) ReadByte(volumePtr VolumePtr) (byte, error) {
	err := v.goToAddress(volumePtr)
	if err != nil {
		return 0, err
	}

	data := make([]byte, 1)
	_, err = v.file.Read(data)
	if err != nil {
		return 0, err
	}

	return data[0], nil
}

func (v Volume) ReadStruct(volumePtr VolumePtr, data interface{}) error {
	err := v.goToAddress(volumePtr)
	if err != nil {
		return err
	}
	err = binary.Read(v.file, v.endianness, data)

	if err != nil {
		return err
	}

	return nil
}

func (v Volume) ReadObject(volumePtr VolumePtr, data interface{}) (VolumeObject, error) {
	err := v.ReadStruct(volumePtr, data)
	if err != nil {
		return VolumeObject{}, nil
	}

	return NewVolumeObject(volumePtr, v, data), nil
}

func (v Volume) Size() (VolumePtr, error) {
	if v.file != nil {
		stat, err := v.file.Stat()
		if err != nil {
			return 0, err
		}

		return VolumePtr(stat.Size()), nil
	}

	return 0, errors.New("Volume file is not opened")
}

func (v Volume) Truncate() error {
	size, err := v.Size()
	if err != nil {
		return err
	}
	err = v.file.Truncate(int64(size))
	if err != nil {
		return err
	}

	return nil
}

func (v Volume) Close() error {
	return v.file.Close()
}

type VolumeObject struct {
	VolumePtr VolumePtr
	Volume    Volume
	Object    interface{}
}

func NewVolumeObject(volumePtr VolumePtr, volume Volume, object interface{}) VolumeObject {
	return VolumeObject{
		VolumePtr: volumePtr,
		Volume:    volume,
		Object:    object,
	}
}

func (vo VolumeObject) Save() error {
	return vo.Volume.WriteStruct(vo.VolumePtr, vo.Object)
}
