package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"os"
	"reflect"
	"testing"
)

type MyStruct struct {
	A int8
	B int16
	C int64
	D int32
	E byte
}

func TestVolumeObjectModification(t *testing.T) {
	path := tempFileName("", "")
	err := vfs.PrepareVolumeFile(path, 1000)

	defer func() {
		_ = os.Remove(path)
	}()

	volume, err := vfs.NewVolume(path)
	if err != nil {
		panic(err)
	}

	defer func() {
		_ = volume.Close()
	}()

	myStruct := MyStruct{
		A: 115,
		B: 3524,
		C: 513651350565461,
		D: 1516516565,
		E: 123,
	}

	err = volume.WriteStruct(53, myStruct)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := volume.ReadObject(53, &MyStruct{})
	if err != nil {
		t.Fatal(err)
	}

	myObj := obj.Object.(*MyStruct)
	myObj.A = 32
	myObj.B = 15
	myObj.D = -54

	err = obj.Save()
	if err != nil {
		t.Fatal(err)
	}

	myReadStruct := MyStruct{}
	err = volume.ReadStruct(53, &myReadStruct)
	if err != nil {
		t.Fatal(err)
	}

	if reflect.DeepEqual(myReadStruct, myObj) {
		t.Error("read and written object are not equal")
	}
}
