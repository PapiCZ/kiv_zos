package commands

import (
	"github.com/PapiCZ/kiv_zos/vfs"
	"github.com/PapiCZ/kiv_zos/vfsapi"
	"github.com/abiosoft/ishell"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func Format(c *ishell.Context) {
	re := regexp.MustCompile("(?P<value>\\d+)(?P<unit>..)")
	submatch := re.FindStringSubmatch(c.Args[0])

	value, err := strconv.Atoi(submatch[1])
	if err != nil {
		c.Err(err)
	}
	unit := strings.ToLower(submatch[2])

	var size vfs.VolumePtr
	switch unit {
	case "kb":
		size = vfs.VolumePtr(value * 1e3)
	case "mb":
		size = vfs.VolumePtr(value * 1e6)
	case "gb":
		size = vfs.VolumePtr(value * 1e9)
	default:
		size = vfs.VolumePtr(value)
	}

	path := "/tmp/vfs.dat"
	err = vfs.PrepareVolumeFile(path, size)

	volume, err := vfs.NewVolume(path)
	if err != nil {
		c.Err(err)
	}

	// Create filesystem
	fs, err := vfs.NewFilesystem(volume, 4096)
	if err != nil {
		c.Err(err)
	}

	err = fs.WriteStructureToVolume()
	if err != nil {
		c.Err(err)
	}

	rootInodeObj, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, true)
	if err != nil {
		c.Err(err)
	}
	rootInode := rootInodeObj.Object.(vfs.Inode)

	err = vfs.InitRootDirectory(&fs, &vfs.MutableInode{
		Inode:    &rootInode,
		InodePtr: vfs.VolumePtrToInodePtr(fs.Superblock, rootInodeObj.VolumePtr),
	})
	if err != nil {
		c.Err(err)
	}

	*(c.Get("fs").(*vfs.Filesystem)) = fs
}

func Mkdir(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	err := vfsapi.Mkdir(*fs, c.Args[0])
	if err != nil {
		c.Err(err)
	}
}

func Ls(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	var path string
	if len(c.Args) == 1 {
		path = c.Args[0]
	} else {
		path = "."
	}

	file, err := vfsapi.Open(*fs, path)
	if err != nil {
		c.Err(err)
	}

	files, err := file.ReadDir()
	if err != nil {
		c.Err(err)
	}

	for _, v := range files {
		if v.IsDir() {
			c.Printf("+ %s\n", v.Name())
		} else {
			c.Printf("- %s\n", v.Name())
		}
	}
}

func Pwd(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	path, err := vfsapi.Abs(*fs, ".")
	if err != nil {
		c.Err(err)
	}
	c.Println(path)
}

func Rmdir(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	// TODO: Check if file is directory

	err := vfsapi.Remove(*fs, c.Args[0])
	if err != nil {
		c.Err(err)
	}
}

func Rm(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	err := vfsapi.Remove(*fs, c.Args[0])
	if err != nil {
		c.Err(err)
	}
}

func Mv(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	src := c.Args[0]
	dst := c.Args[1]

	// If dst exists and is directory, copy src into that directory
	dstExists, err := vfsapi.Exists(*fs, dst)
	if err != nil {
		c.Err(err)
	}
	if dstExists {
		srcFragments := strings.Split(src, "/")
		dst += "/" + srcFragments[len(srcFragments)-1]
	}

	err = vfsapi.Rename(*fs, src, dst)
	if err != nil {
		c.Err(err)
	}
}

func Cp(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	src := c.Args[0]
	dst := c.Args[1]

	// If dst exists and is directory, copy src into that directory
	dstExists, err := vfsapi.Exists(*fs, dst)
	if err != nil {
		c.Err(err)
		return
	}
	if dstExists {
		srcFragments := strings.Split(src, "/")
		dst += "/" + srcFragments[len(srcFragments)-1]
	}

	// Open source file in virtual filesystem
	srcFile, err := vfsapi.Open(*fs, src)
	if err != nil {
		c.Err(err)
		return
	}

	// Open destination file in virtual filesystem
	dstFile, err := vfsapi.Open(*fs, dst)
	if err != nil {
		c.Err(err)
		return
	}

	if srcFile.IsDir() {
		c.Println("DIRECTORY CANNOT BE COPIED")
		return
	}

	// Copy data
	data := make([]byte, 4000)
	for {
		n, err := srcFile.Read(data)
		if err != nil {
			if err == io.EOF {
				break
			}
			c.Err(err)
			return
		}

		data = data[:n]
		n, err = dstFile.Write(data)
		if err != nil {
			c.Err(err)
			return
		}
	}
}

func Cd(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	err := vfsapi.ChangeDirectory(fs, c.Args[0])
	if err != nil {
		c.Err(err)
	}

	absPath, err := vfsapi.Abs(*fs, ".")
	if err != nil {
		c.Err(err)
	}

	c.SetPrompt(absPath + " > ")
}

func Incp(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	hostSrc := c.Args[0]
	vfsDst := c.Args[1]

	// Open file in host filesystem
	srcFile, err := os.Open(hostSrc)
	if err != nil {
		c.Err(err)
		return
	}
	defer func() {
		_ = srcFile.Close()
	}()

	// Open file in virtual filesystem
	dstFile, err := vfsapi.Open(*fs, vfsDst)
	if err != nil {
		c.Err(err)
		return
	}

	// Copy data
	data := make([]byte, 4000)
	for {
		n, err := srcFile.Read(data)
		if err != nil {
			if err == io.EOF {
				break
			}
			c.Err(err)
			return
		}

		data = data[:n]
		n, err = dstFile.Write(data)
		if err != nil {
			c.Err(err)
			return
		}
	}
}

func Outcp(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	vfsSrc := c.Args[0]
	hostDst := c.Args[1]

	// Open file in virtual filesystem
	srcFile, err := vfsapi.Open(*fs, vfsSrc)
	if err != nil {
		c.Err(err)
		return
	}

	// Open file in host filesystem
	dstFile, err := os.Create(hostDst)
	if err != nil {
		c.Err(err)
		return
	}
	defer func() {
		_ = dstFile.Close()
	}()

	// Copy data
	data := make([]byte, 4000)
	for {
		n, err := srcFile.Read(data)
		if err != nil {
			if err == io.EOF {
				break
			}
			c.Err(err)
			return
		}

		data = data[:n]
		n, err = dstFile.Write(data)
		if err != nil {
			c.Err(err)
			return
		}
	}
}

func Cat(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)
	path := c.Args[0]

	file, err := vfsapi.Open(*fs, path)
	if err != nil {
		c.Err(err)
	}

	data := make([]byte, 4000)
	for {
		n, err := file.Read(data)
		if err != nil {
			if err == io.EOF {
				break
			}
			c.Err(err)
			return
		}

		data = data[:n]
		c.Printf("%s", data)
	}
}

func Info(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	directPtrs, indirect1Ptrs, indirect2Ptrs, err := vfsapi.DataClustersInfo(*fs, c.Args[0])
	if err != nil {
		c.Err(err)
	}

	c.Println("Direct pointers")
	c.Println(strings.Join(ClusterPtrsToStrings(directPtrs), " "))

	c.Println("\nIndirect1 pointers")
	for k, v := range indirect1Ptrs {
		c.Printf("%d -> ", k)
		c.Println(strings.Join(ClusterPtrsToStrings(v), " "))
	}

	c.Println("\nIndirect2 pointers")
	for k, v := range indirect2Ptrs {
		c.Printf("%d ->\n", k)
		for k2, v2 := range v {
			c.Printf("\t%d -> ", k2)
			c.Println(strings.Join(ClusterPtrsToStrings(v2), " "))
		}
		c.Println()
	}
}

func Check(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	err := vfsapi.FsCheck(*fs)
	if err != nil {
		c.Err(err)
	}
}
