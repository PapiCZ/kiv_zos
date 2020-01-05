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

func TestNeededMemoryForBitmap(t *testing.T) {
	if vfs.NeededMemoryForBitmap(63) != 8 {
		t.Errorf("needed memory is %d instead of %d", vfs.NeededMemoryForBitmap(63), 8)
	}
}

func TestZerosCount(t *testing.T) {
	b := vfs.NewBitmap(10)
	err := b.SetBit(9, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetBit(5, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetBit(3, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetBit(7, 1)
	if err != nil {
		t.Fatal(err)
	}

	if b.Zeros() != 12 {
		// 12 because we allocated 2 bytes
		t.Errorf("number of zeros should be 12 instead of %d", b.Zeros())
	}
}

func TestOnesCount(t *testing.T) {
	b := vfs.NewBitmap(10)
	err := b.SetBit(9, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetBit(5, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetBit(3, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = b.SetBit(7, 1)
	if err != nil {
		t.Fatal(err)
	}

	if b.Ones() != 4 {
		t.Errorf("number of zeros should be 4 instead of %d", b.Ones())
	}
}
