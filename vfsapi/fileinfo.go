package vfsapi

type FileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (fi FileInfo) Name() string {
	return fi.name
}

func (fi FileInfo) Size() int64 {
	return fi.size
}

func (fi FileInfo) IsDir() bool {
	return fi.isDir
}
