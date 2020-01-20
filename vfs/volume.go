package vfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const CachePageSize = 1024

type VolumePtr int64
type ClusterPtr int32
type InodePtr int32

type Volume struct {
	file       *os.File
	endianness binary.ByteOrder
	position   VolumePtr
}

func PrepareVolumeFile(path string, size VolumePtr) error {
	f, err := os.Create(path)

	defer func() {
		_ = f.Close()
	}()

	if err != nil {
		return err
	}

	err = f.Truncate(int64(size))
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

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return Volume{}, err
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return Volume{}, err
	}

	return Volume{
		file:       f,
		endianness: binary.LittleEndian,
		position:   0,
	}, nil
}

func (v *Volume) goToAddress(volumePtr VolumePtr) error {
	_, err := v.file.Seek(int64(volumePtr), io.SeekStart)
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

	return 0, errors.New("missing volume file")
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

func (v Volume) Destroy() error {
	_ = v.Close()
	return os.Remove(v.file.Name())
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

type PageMissingError struct {
	requestedPage int64
}

func (p PageMissingError) Error() string {
	return fmt.Sprintf("requested page %d is not in cachedPages", p.requestedPage)
}

type CachedVolume struct {
	Volume      Volume
	cachedPages map[int64][CachePageSize]byte
}

func (cv *CachedVolume) readFromCache(volumePtr VolumePtr, data interface{}) error {
	requestedBytes := int64(binary.Size(data))
	virtualVolume := make([]byte, 0)

	pageIndex := int64(volumePtr) / CachePageSize
	pageOffset := int64(volumePtr) % CachePageSize
	for {
		// Do we have requested page in cachedPages?
		page, ok := cv.cachedPages[pageIndex]

		if !ok {
			return PageMissingError{pageIndex}
		}

		if requestedBytes <= CachePageSize-pageOffset {
			virtualVolume = append(virtualVolume, page[pageOffset:requestedBytes]...)
			requestedBytes = 0
			break
		} else {
			// We need another page
			pageSlice := page[pageOffset:]
			virtualVolume = append(virtualVolume, pageSlice...)
			requestedBytes -= int64(len(pageSlice))
			pageIndex++
			pageOffset = 0
		}
	}

	reader := bytes.NewReader(virtualVolume)
	err := binary.Read(reader, binary.LittleEndian, data)
	if err != nil {
		return err
	}

	return nil
}

func (cv *CachedVolume) loadPageIntoCache(pageIndex int64) error {
	data := [CachePageSize]byte{}

	err := cv.Volume.ReadStruct(VolumePtr(pageIndex*CachePageSize), &data)
	if err != nil {
		return err
	}

	cv.cachedPages[pageIndex] = data

	return nil
}
