package tests

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/PapiCZ/kiv_zos/vfs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func tempFileName(prefix, suffix string) string {
	randBytes := make([]byte, 16)
	_, _ = rand.Read(randBytes)
	return filepath.Join(os.TempDir(), prefix+hex.EncodeToString(randBytes)+suffix)
}

func writeAndReadStruct(address int32, sourceData interface{}, targetData interface{}) {
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

	err = volume.WriteStruct(address, sourceData)
	if err != nil {
		panic(err)
	}

	err = volume.ReadStruct(address, targetData)
	if err != nil {
		panic(err)
	}
}

func TestSuperblockWriteAndRead(t *testing.T) {
	var signature [9]byte
	copy(signature[:], "janopa")

	var volumeDescriptor [251]byte
	copy(volumeDescriptor[:], "kiv/zos")

	dataIn := vfs.NewPreparedSuperblock("janopa", "kiv/zos", 1000, 512)
	dataOut := vfs.Superblock{}

	writeAndReadStruct(0, &dataIn, &dataOut)

	if !reflect.DeepEqual(dataIn, dataOut) {
		t.Fatalf("\n%#v\n--- IS NOT SAME AS ---\n%#v", dataIn, dataOut)
	}
}

func TestBitmapWriteAndRead(t *testing.T) {
	dataIn := vfs.NewBitmap(16)
	err := dataIn.SetBit(5, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = dataIn.SetBit(9, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = dataIn.SetBit(13, 1)
	if err != nil {
		t.Fatal(err)
	}
	dataOut := make(vfs.Bitmap, 2)

	writeAndReadStruct(0, &dataIn, &dataOut)

	if !reflect.DeepEqual(dataIn, dataOut) {
		t.Fatalf("\n%#v\n--- IS NOT SAME AS ---\n%#v", dataIn, dataOut)
	}
}
