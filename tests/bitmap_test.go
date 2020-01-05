package tests

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"testing"
)

func TestSetAndGetBit(t *testing.T) {
	bitmap := make(vfs.Bitmap, 2)
	err := bitmap.SetBit(0, 1)
	if err != nil {
		t.Fatal(err)
	}

	val, err := bitmap.GetBit(0)
	if err != nil {
		t.Fatal(err)
	}

	if val != 1 {
		t.Fatal("written and read byte is not same!")
	}
}

func TestSetOverwriteZeroAndGetBit(t *testing.T) {
	bitmap := vfs.NewBitmap(2)
	err := bitmap.SetBit(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = bitmap.SetBit(0, 0)
	if err != nil {
		t.Fatal(err)
	}


	val, err := bitmap.GetBit(0)
	if err != nil {
		t.Fatal(err)
	}

	if val != 0 {
		t.Fatal("written and read byte is not same!")
	}
}

func TestSetOverwriteOneAndGetBit(t *testing.T) {
	bitmap := vfs.NewBitmap(2)
	err := bitmap.SetBit(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	err = bitmap.SetBit(0, 1)
	if err != nil {
		t.Fatal(err)
	}


	val, err := bitmap.GetBit(0)
	if err != nil {
		t.Fatal(err)
	}

	if val != 1 {
		t.Fatal("written and read byte is not same!")
	}
}

func TestOutOfRangeFail(t *testing.T) {
	bitmap := vfs.NewBitmap(8)
	_, err := bitmap.GetBit(8)

	switch err.(type) {
	case vfs.OutOfRange:
		return
	default:
		t.Fatalf("bad error: %s\nexpected OutOfRange", err)
	}
}
