package vfsapi

import "github.com/PapiCZ/kiv_zos/vfs"

type File struct {
	mutableInode vfs.MutableInode
	ptrOffset    int64
}

func Open(name string) File {

}
