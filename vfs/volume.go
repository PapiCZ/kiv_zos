package vfs

import (
	"encoding/binary"
	"errors"
	"os"
)

type Vptr int64
type Cptr int32

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

func (v *Volume) goToAddress(address Vptr) error {
	_, err := v.file.Seek(int64(address), 0)
	if err != nil {
		return err
	}

	return nil
}

func (v Volume) WriteStruct(address Vptr, data interface{}) error {
	err := v.goToAddress(address)
	if err != nil {
		return err
	}
	err = binary.Write(v.file, v.endianness, data)
	if err != nil {
		return err
	}

	return nil
}

func (v Volume) ReadStruct(address Vptr, data interface{}) error {
	err := v.goToAddress(address)
	if err != nil {
		return err
	}
	err = binary.Read(v.file, v.endianness, data)

	if err != nil {
		return err
	}

	return nil
}

func (v Volume) ReadObject(address Vptr, data interface{}) (VolumeObject, error) {
	err := v.ReadStruct(address, data)
	if err != nil {
		return VolumeObject{}, nil
	}

	return NewVolumeObject(address, v, data), nil
}

func (v Volume) Size() (Vptr, error) {
	if v.file != nil {
		stat, err := v.file.Stat()
		if err != nil {
			return 0, err
		}

		return Vptr(stat.Size()), nil
	}

	return 0, errors.New("volume file is not opened")
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
	address Vptr
	volume  Volume
	Object  interface{}
}

func NewVolumeObject(address Vptr, volume Volume, object interface{}) VolumeObject {
	return VolumeObject{
		address: address,
		volume:  volume,
		Object:  object,
	}
}

func (vo VolumeObject) Save() error {
	return vo.volume.WriteStruct(vo.address, vo.Object)
}
