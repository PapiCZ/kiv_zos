package vfs

import (
	"encoding/binary"
	"errors"
	"os"
)

type Volume struct {
	file           *os.File
	endianness     binary.ByteOrder
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

func (v *Volume) goToAddress(address int32) error {
	_, err := v.file.Seek(int64(address), 0)
	if err != nil {
		return err
	}

	return nil
}

func (v Volume) WriteStruct(address int32, data interface{}) error {
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

func (v Volume) ReadStruct(address int32, data interface{}) error {
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

func (v Volume) Size() (int64, error) {
	if v.file != nil {
		stat, err := v.file.Stat()
		if err != nil {
			return 0, err
		}

		return stat.Size(), nil
	}

	return 0, errors.New("Volume file is not opened")
}

func (v Volume) Truncate() error {
	size, err := v.Size()
	if err != nil {
		return err
	}
	err = v.file.Truncate(size)
	if err != nil {
		return err
	}

	return nil
}

func (v Volume) Close() error {
	return v.file.Close()
}
