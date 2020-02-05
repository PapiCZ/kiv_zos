package main

import (
	"fmt"
	"github.com/PapiCZ/kiv_zos/shell"
	"github.com/PapiCZ/kiv_zos/vfs"
	"github.com/abiosoft/ishell"
	"os"
)

func main() {
	s := ishell.New()
	s.SetPrompt("/ > ")
	s.Set("volume_path", os.Args[1])
	s.Set("fs", &vfs.Filesystem{})
	s.Set("s", s)

	path := s.Get("volume_path").(string)
	_, err := os.Stat(path)
	if err == nil {
		// We want to load existing filesystem volume
		volume, err := vfs.NewVolume(path)
		if err != nil {
			fmt.Println(err)
		}

		// Read superblock
		var sb vfs.Superblock
		err = volume.ReadStruct(0, &sb)
		if err != nil {
			fmt.Println(err)
			return
		}

		// Create filesystem
		fs := vfs.NewFilesystemFromSuperblock(volume, sb)

		*(s.Get("fs").(*vfs.Filesystem)) = fs
	}

	s.AddCmd(&ishell.Cmd{
		Name:      "format",
		Func:      shell.Format,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "mkdir",
		Func:      shell.Mkdir,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "ls",
		Func:      shell.Ls,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "rmdir",
		Func:      shell.Rmdir,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "rm",
		Func:      shell.Rm,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "badrm",
		Func:      shell.Badrm,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "mv",
		Func:      shell.Mv,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "cd",
		Func:      shell.Cd,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "cp",
		Func:      shell.Cp,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "incp",
		Func:      shell.Incp,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "outcp",
		Func:      shell.Outcp,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "pwd",
		Func:      shell.Pwd,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "cat",
		Func:      shell.Cat,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "info",
		Func:      shell.Info,
		Completer: nil,
	})


	s.AddCmd(&ishell.Cmd{
		Name:      "check",
		Func:      shell.Check,
		Completer: nil,
	})

	s.AddCmd(&ishell.Cmd{
		Name:      "load",
		Func:      shell.Load,
		Completer: nil,
	})

	s.Run()
}
