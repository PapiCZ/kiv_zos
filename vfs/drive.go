package vfs

import (
	"encoding/binary"
	"os"
)

type Address int64

type Drive struct {
	file           *os.File
	endianness     binary.ByteOrder
	currentAddress Address
}

func PrepareDriveFile(path string, size int64) error {
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

func NewDrive(path string) (*Drive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &Drive{
		file:       f,
		endianness: binary.LittleEndian,
	}, nil
}

func (d *Drive) goToAddress(address Address) error {
	newAddress, err := d.file.Seek(int64(address), 0)
	if err != nil {
		return err
	}

	d.currentAddress = Address(newAddress)

	return nil
}

func (d Drive) WriteStruct(address Address, data interface{}) error {
	err := binary.Write(d.file, d.endianness, data)
	if err != nil {
		return err
	}

	return nil
}

func (d Drive) ReadStruct(address Address, data interface{}) error {
	err := binary.Read(d.file, d.endianness, data)

	if err != nil {
		return err
	}

	return nil
}

func (d Drive) Close() error {
	return d.file.Close()
}
