package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"math/rand"
	"os"
	"reflect"
	"testing"
)

func CreateVolume(t *testing.T) vfs.Volume {
	path := tempFileName("", "")
	err := vfs.PrepareVolumeFile(path, 1e7)

	defer func() {
		_ = os.Remove(path)
	}()

	volume, err := vfs.NewVolume(path)
	if err != nil {
		t.Fatal(err)
	}

	return volume
}

type Foo struct {
	A [3]byte
	B int64
	C [2050]byte
	D [2]byte
}

func TestCachedVolumeWrite(t *testing.T) {
	volume := CreateVolume(t)
	defer func() {
		_ = volume.Destroy()
	}()

	cachedVolume := vfs.NewCachedVolume(volume)

	aRand := [3]byte{}
	cRand := [2050]byte{}
	dRand := [2]byte{}

	temp := make([]byte, 3)
	_, err := rand.Read(temp)
	if err != nil {
		t.Fatal(err)
	}
	copy(aRand[:], temp)

	temp = make([]byte, 2050)
	_, err = rand.Read(temp)
	if err != nil {
		t.Fatal(err)
	}
	copy(cRand[:], temp)

	temp = make([]byte, 2)
	_, err = rand.Read(temp)
	if err != nil {
		t.Fatal(err)
	}
	copy(dRand[:], temp)

	data := Foo{
		A: aRand,
		B: rand.Int63(),
		C: cRand,
		D: dRand,
	}

	err = cachedVolume.WriteStruct(2, data)
	if err != nil {
		t.Fatal(err)
	}

	err = cachedVolume.Flush()
	if err != nil {
		t.Fatal(err)
	}

	readData := Foo{}
	err = volume.ReadStruct(2, &readData)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(data, readData) {
		t.Error("read and written data are not equal")
	}
}
