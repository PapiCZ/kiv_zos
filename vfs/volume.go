package vfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
)

const CachePageSize = 1024

type VolumePtr int64
type ClusterPtr int32
type InodePtr int32

type ReadableVolume interface {
	ReadByte(volumePtr VolumePtr) (byte, error)
	ReadBytes(volumePtr VolumePtr, data []byte) error
	ReadStruct(volumePtr VolumePtr, data interface{}) error
	ReadObject(volumePtr VolumePtr, data interface{}) (VolumeObject, error)
}

type WritableVolume interface {
	WriteStruct(volumePtr VolumePtr, data interface{}) error
	WriteByte(volumePtr VolumePtr, data byte) error
}

type ReadWriteVolume interface {
	WritableVolume
	ReadableVolume
}

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

func(v Volume) ReadBytes(volumePtr VolumePtr, data []byte) error {
	err := v.goToAddress(volumePtr)
	if err != nil {
		return err
	}

	_, err = v.file.Read(data)
	if err != nil {
		return err
	}

	return nil
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
	Volume    ReadWriteVolume
	Object    interface{}
}

func NewVolumeObject(volumePtr VolumePtr, volume ReadWriteVolume, object interface{}) VolumeObject {
	return VolumeObject{
		VolumePtr: volumePtr,
		Volume:    volume,
		Object:    object,
	}
}

func (vo VolumeObject) Save() error {
	return vo.Volume.WriteStruct(vo.VolumePtr, vo.Object)
}

type CachedVolume struct {
	Volume      ReadWriteVolume
	cachedPages map[int64]*[CachePageSize]byte
}

func NewCachedVolume(volume ReadWriteVolume) CachedVolume {
	return CachedVolume{
		Volume:      volume,
		cachedPages: make(map[int64]*[CachePageSize]byte),
	}
}

func (cv *CachedVolume) readFromCache(volumePtr VolumePtr, data interface{}) error {
	var requestedBytes int64
	switch data.(type) {
	case []byte:
		requestedBytes = int64(len(data.([]byte)))
	default:
		requestedBytes = int64(binary.Size(data))
	}
	virtualVolume := make([]byte, 0)

	pageIndex := int64(volumePtr) / CachePageSize
	pageOffset := int64(volumePtr) % CachePageSize
	for {
		// Do we have requested page in cachedPages?
		page, ok := cv.cachedPages[pageIndex]

		if !ok {
			// Load missing page
			err := cv.loadPageIntoCache(pageIndex)
			if err != nil {
				return err
			}

			page = cv.cachedPages[pageIndex]
		}

		if requestedBytes <= CachePageSize-pageOffset {
			if len(virtualVolume) == 0 {
				virtualVolume = page[pageOffset:pageOffset+requestedBytes]
			} else {
				virtualVolume = append(virtualVolume, page[pageOffset:pageOffset+requestedBytes]...)
			}
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

	switch data.(type) {
	case []byte:
		copy(data.([]byte), virtualVolume)
	default:
		reader := bytes.NewReader(virtualVolume)
		err := binary.Read(reader, binary.LittleEndian, data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cv *CachedVolume) writeToCache(volumePtr VolumePtr, data interface{}) error {
	remainingBytes := int64(binary.Size(data))
	virtualVolume := new(bytes.Buffer)
	err := binary.Write(virtualVolume, binary.LittleEndian, data)
	if err != nil {
		return err
	}

	pageIndex := int64(volumePtr) / CachePageSize
	pageOffset := int64(volumePtr) % CachePageSize

	for {
		page, ok := cv.cachedPages[pageIndex]

		if !ok {
			// Load missing page
			err := cv.loadPageIntoCache(pageIndex)
			if err != nil {
				return err
			}

			page = cv.cachedPages[pageIndex]
		}

		if remainingBytes <= CachePageSize-pageOffset {
			buf := make([]byte, remainingBytes)
			n, err := virtualVolume.Read(buf)
			if err != nil {
				return err
			}
			copy(page[pageOffset:pageOffset+int64(n)], buf)
			remainingBytes = 0
			break
		} else {
			// We need another page
			bufSize := int(math.Min(float64(remainingBytes), CachePageSize)) - int(pageOffset)
			if bufSize < 0 {
				// TODO: Weird fix
				bufSize = int(remainingBytes)
			}
			buf := make([]byte, bufSize)
			_, err = virtualVolume.Read(buf)
			if err != nil {
				return err
			}

			copy(page[pageOffset:], buf)
			remainingBytes -= int64(bufSize)
			pageIndex++
			pageOffset = 0
		}
	}

	return nil
}

func (cv *CachedVolume) loadPageIntoCache(pageIndex int64) error {
	data := [CachePageSize]byte{}

	err := cv.Volume.ReadStruct(VolumePtr(pageIndex*CachePageSize), &data)
	if err != nil {
		return err
	}

	cv.cachedPages[pageIndex] = &data

	return nil
}

func (cv *CachedVolume) loadPagesIntoCache(pageIndexes ...int64) error {
	for _, pageIndex := range pageIndexes {
		err := cv.loadPageIntoCache(pageIndex)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cv CachedVolume) getPageIndexes(startPageIndex int64, length int) []int64 {
	remainingLength := length
	pageIndexes := make([]int64, 0)

	pageIndex := startPageIndex
	for {
		if remainingLength > 0 {
			pageIndexes = append(pageIndexes, startPageIndex)
			pageIndex++
			remainingLength -= CachePageSize
		} else {
			break
		}
	}

	return pageIndexes
}

func (cv *CachedVolume) writePageToVolume(pageIndex int64) error {
	page := cv.cachedPages[pageIndex]

	err := cv.Volume.WriteStruct(VolumePtr(pageIndex*CachePageSize), page)
	if err != nil {
		return err
	}

	return nil
}

func (cv CachedVolume) Flush() error {
	for pageIndex, _ := range cv.cachedPages {
		err := cv.writePageToVolume(pageIndex)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cv CachedVolume) WriteStruct(volumePtr VolumePtr, data interface{}) error {
	err := cv.writeToCache(volumePtr, data)
	if err != nil {
		return err
	}

	return nil
}

func (cv CachedVolume) WriteByte(volumePtr VolumePtr, data byte) error {
	err := cv.writeToCache(volumePtr, data)
	if err != nil {
		return err
	}

	return nil
}

func (cv CachedVolume) ReadByte(volumePtr VolumePtr) (byte, error) {
	// We can't use readFromCache here. It's too expensive.
	pageIndex := int64(volumePtr) / CachePageSize
	pageOffset := int64(volumePtr) % CachePageSize
	page, ok := cv.cachedPages[pageIndex]

	if !ok {
		// Load missing page
		err := cv.loadPageIntoCache(pageIndex)
		if err != nil {
			return 0, err
		}

		page = cv.cachedPages[pageIndex]
	}

	return page[pageOffset], nil
}

func(cv CachedVolume) ReadBytes(volumePtr VolumePtr, data []byte) error {
	err := cv.readFromCache(volumePtr, data)
	if err != nil {
		return err
	}

	return nil
}


func (cv CachedVolume) ReadStruct(volumePtr VolumePtr, data interface{}) error {
	err := cv.readFromCache(volumePtr, data)
	if err != nil {
		return err
	}

	return nil
}

func (cv CachedVolume) ReadObject(volumePtr VolumePtr, data interface{}) (VolumeObject, error) {
	err := cv.readFromCache(volumePtr, data)
	if err != nil {
		return VolumeObject{}, nil
	}

	return NewVolumeObject(volumePtr, cv, data), nil
}
