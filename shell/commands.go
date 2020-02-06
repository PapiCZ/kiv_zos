package shell

import (
	"fmt"
	"github.com/PapiCZ/kiv_zos/vfs"
	"github.com/PapiCZ/kiv_zos/vfsapi"
	"github.com/abiosoft/ishell"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func Format(c *ishell.Context) {
	if len(c.Args) != 1 {
		c.Println("expected 1 argument")
		return
	}

	re := regexp.MustCompile("(?P<value>\\d+)(?P<unit>.{0,2})")
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

	if size < 1e6 {
		c.Println("MINIMUM FILESYSTEM SIZE IS 1MB")
		return
	}

	path := c.Get("volume_path").(string)
	// Create new filesystem
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
	if len(c.Args) != 1 {
		c.Println("expected 1 argument")
		return
	}

	fs := c.Get("fs").(*vfs.Filesystem)


	err := vfsapi.Mkdir(*fs, c.Args[0])
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("PATH NOT FOUND (neexistuje zadaná cesta)")
		case vfs.DuplicateDirectoryEntry:
			c.Println("EXIST (nelze založit, již existuje)")
		default:
			c.Err(err)
		}
		return
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

	file, err := vfsapi.Open(*fs, path, false)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("PATH NOT FOUND (neexistující adresář)")
		default:
			c.Err(err)
		}
		return
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
	if len(c.Args) != 1 {
		c.Println("expected 1 argument")
		return
	}

	fs := c.Get("fs").(*vfs.Filesystem)

	path := c.Args[0]
	file, err := vfsapi.Open(*fs, path, false)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("FILE NOT FOUND (neexistující adresář)")
		default:
			c.Err(err)
		}
		return
	}
	if !file.IsDir() {
		c.Println("NOT A DIRECTORY")
		return
	}

	files, err := file.ReadDir()
	if err != nil {
		c.Err(err)
		return
	}

	if len(files) > 2 {
		c.Println("NOT EMPTY (adresář obsahuje podadresáře, nebo soubory)")
		return
	}

	err = vfsapi.Remove(*fs, path)
	if err != nil {
		c.Err(err)
	}
}

func Rm(c *ishell.Context) {
	if len(c.Args) != 1 {
		c.Println("expected 1 argument")
		return
	}

	fs := c.Get("fs").(*vfs.Filesystem)

	path := c.Args[0]
	file, err := vfsapi.Open(*fs, path, false)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("FILE NOT FOUND")
		default:
			c.Err(err)
		}
		return
	}
	if file.IsDir() {
		c.Println("CANNOT REMOVE DIRECTORY (use rmdir instead)")
		return
	}

	err = vfsapi.Remove(*fs, path)
	if err != nil {
		c.Err(err)
		return
	}
	c.Println("OK")
}

func Badrm(c *ishell.Context) {
	if len(c.Args) != 1 {
		c.Println("expected 1 argument")
		return
	}

	fs := c.Get("fs").(*vfs.Filesystem)

	path := c.Args[0]
	file, err := vfsapi.Open(*fs, path, false)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("FILE NOT FOUND")
		default:
			c.Err(err)
		}
		return
	}
	if file.IsDir() {
		c.Println("CANNOT REMOVE DIRECTORY (use rmdir instead)")
		return
	}

	err = vfsapi.BadRemove(*fs, path)
	if err != nil {
		c.Err(err)
		return
	}
	c.Println("OK")
}


func Mv(c *ishell.Context) {
	if len(c.Args) != 2 {
		c.Println("expected 2 arguments")
		return
	}

	fs := c.Get("fs").(*vfs.Filesystem)

	src := c.Args[0]
	dst := c.Args[1]

	srcExists, err := vfsapi.Exists(*fs, src)
	if err != nil {
		c.Err(err)
		return
	}

	if !srcExists {
		c.Println("FILE NOT FOUND (není zdroj)")
		return
	}

	// If dst exists and is directory, copy src into that directory
	dstFile, err := vfsapi.Open(*fs, dst, false)
	if err == nil && dstFile.IsDir() {
		srcFragments := strings.Split(src, "/")
		dst += "/" + srcFragments[len(srcFragments)-1]
	} else if err == nil && !dstFile.IsDir() {
		err = vfsapi.Remove(*fs, dst)
		if err != nil {
			c.Err(err)
		}
	}

	err = vfsapi.Rename(*fs, src, dst)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("PATH NOT FOUND (neexistuje cílová cesta)")
		default:
			c.Err(err)
		}
		return
	}

	c.Println("OK")
}

func Cp(c *ishell.Context) {
	if len(c.Args) != 2 {
		c.Println("expected 2 arguments")
		return
	}

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
	srcExists, err := vfsapi.Exists(*fs, src)
	if err != nil {
		c.Err(err)
		return
	}

	if !srcExists {
		c.Println("FILE NOT FOUND (není zdroj)")
		return
	}

	srcFile, err := vfsapi.Open(*fs, src, false)
	if err != nil {
		c.Err(err)
		return
	}

	if srcFile.IsDir() {
		c.Println("DIRECTORY CANNOT BE COPIED")
		return
	}

	// Open destination file in virtual filesystem
	dstFileExists, _ := vfsapi.Exists(*fs, dst)
	if dstFileExists {
		_ = vfsapi.Remove(*fs, dst)
	}

	dstFile, err := vfsapi.Open(*fs, dst, true)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("PATH NOT FOUND (neexistuje cílová cesta)")
		default:
			c.Err(err)
		}
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
			switch err.(type) {
			case vfs.ClusterIndexOutOfRange:
				fmt.Println("NOT ENOUGH AVAILABLE SPACE")
			default:
				c.Err(err)
			}
			return
		}
	}

	c.Println("OK")
}

func Cd(c *ishell.Context) {
	if len(c.Args) != 1 {
		c.Println("expected 1 argument")
		return
	}

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
	if len(c.Args) != 2 {
		c.Println("expected 2 arguments")
		return
	}

	fs := c.Get("fs").(*vfs.Filesystem)

	hostSrc := c.Args[0]
	vfsDst := c.Args[1]

	// Open file in host filesystem
	srcFile, err := os.Open(hostSrc)
	if err != nil {
		if os.IsNotExist(err) {
			c.Println("FILE NOT FOUND (není zdroj)")
		} else {
			c.Err(err)
		}
		return
	}
	defer func() {
		_ = srcFile.Close()
	}()

	// Open file in virtual filesystem
	dstFile, err := vfsapi.Open(*fs, vfsDst, true)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("PATH NOT FOUND (neexistuje cílová cesta)")
		default:
			c.Err(err)
		}
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
			switch err.(type) {
			case vfs.ClusterIndexOutOfRange:
				fmt.Println("NOT ENOUGH AVAILABLE SPACE")
			default:
				c.Err(err)
			}
			return
		}
	}
}

func Outcp(c *ishell.Context) {
	if len(c.Args) != 2 {
		c.Println("expected 2 arguments")
		return
	}

	fs := c.Get("fs").(*vfs.Filesystem)

	vfsSrc := c.Args[0]
	hostDst := c.Args[1]

	// Open file in virtual filesystem
	srcFile, err := vfsapi.Open(*fs, vfsSrc, false)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("FILE NOT FOUND (není zdroj)")
		default:
			c.Err(err)
		}
		return
	}

	// Open file in host filesystem
	dstFile, err := os.Create(hostDst)
	if err != nil {
		if os.IsNotExist(err) {
			c.Println("FILE NOT FOUND (není zdroj)")
		} else {
			c.Err(err)
		}
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
	if len(c.Args) != 1 {
		c.Println("expected 1 argument")
		return
	}

	fs := c.Get("fs").(*vfs.Filesystem)
	path := c.Args[0]

	file, err := vfsapi.Open(*fs, path, false)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("FILE NOT FOUND (není zdroj)")
		default:
			c.Err(err)
		}
		return
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
	if len(c.Args) != 1 {
		c.Println("expected 1 argument")
		return
	}

	fs := c.Get("fs").(*vfs.Filesystem)

	path := c.Args[0]
	file, err := vfsapi.Open(*fs, path, false)
	if err != nil {
		switch err.(type) {
		case vfs.DirectoryEntryNotFound:
			c.Println("FILE NOT FOUND (není zdroj)")
		default:
			c.Err(err)
		}
		return
	}

	directPtrs, indirect1Ptrs, indirect2Ptrs, err := vfsapi.DataClustersInfo(*fs, path)
	if err != nil {
		c.Err(err)
		return
	}

	c.Printf("%s - %d - %d\n", file.Name(), file.Size(), file.InodePtr())
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

func Load(c *ishell.Context) {
	if len(c.Args) != 1 {
		c.Println("expected 1 arguments")
		return
	}

	shell := c.Get("shell").(*ishell.Shell)

	path := c.Args[0]

	// Open file on host filesystem
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.Println("FILE NOT FOUND (není zdroj)")
		} else {
			c.Err(err)
		}
		return
	}

	for _, cmd := range strings.Split(string(bytes), "\n") {
		if len(cmd) == 0 {
			continue
		}
		fmt.Println(cmd)

		err = shell.Process(strings.Split(cmd, " ")...)
		if err != nil {
			c.Err(err)
			return
		}
	}
}
