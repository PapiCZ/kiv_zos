package vfsapi

type FileInfo struct {
	name  string
	size  int
	isDir bool
}

func (fi FileInfo) Name() string {
	return fi.name
}

func (fi FileInfo) Size() int {
	return fi.size
}

func (fi FileInfo) IsDir() bool {
	return fi.isDir
}
