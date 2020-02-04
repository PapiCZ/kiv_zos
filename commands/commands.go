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

	vo, err := vfs.FindFreeInode(fs.Volume, fs.Superblock, false)
	if err != nil {
		c.Err(err)
	}

	_, err = vfs.IsInodeFree(vo.Volume, fs.Superblock, vfs.VolumePtrToInodePtr(fs.Superblock, vo.VolumePtr))
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

	err := vfsapi.Rename(*fs, c.Args[0], c.Args[1])
	if err != nil {
		c.Err(err)
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

func Cp(c *ishell.Context) {
	fs := c.Get("fs").(*vfs.Filesystem)

	srcPath := c.Args[0]
	dstPath := c.Args[1]

	// Open source file in virtual filesystem
	srcFile, err := vfsapi.Open(*fs, srcPath)
	if err != nil {
		c.Err(err)
		return
	}

	// Open destination file in virtual filesystem
	dstFile, err := vfsapi.Open(*fs, dstPath)
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
