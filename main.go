package main

import (
	"github.com/PapiCZ/kiv_zos/commands"
	"github.com/PapiCZ/kiv_zos/vfs"
	"github.com/abiosoft/ishell"
)

func main() {
	shell := ishell.New()
	shell.SetPrompt("/ > ")
	shell.Set("fs", &vfs.Filesystem{})

	shell.AddCmd(&ishell.Cmd{
		Name:      "format",
		Func:      commands.Format,
		Completer: nil,
	})

	shell.AddCmd(&ishell.Cmd{
		Name:      "mkdir",
		Func:      commands.Mkdir,
		Completer: nil,
	})

	shell.AddCmd(&ishell.Cmd{
		Name:      "ls",
		Func:      commands.Ls,
		Completer: nil,
	})

	shell.AddCmd(&ishell.Cmd{
		Name:      "rmdir",
		Func:      commands.Rmdir,
		Completer: nil,
	})

	shell.AddCmd(&ishell.Cmd{
		Name:      "rm",
		Func:      commands.Rm,
		Completer: nil,
	})

	shell.AddCmd(&ishell.Cmd{
		Name:      "mv",
		Func:      commands.Mv,
		Completer: nil,
	})

	shell.AddCmd(&ishell.Cmd{
		Name:      "cd",
		Func:      commands.Cd,
		Completer: nil,
	})

	shell.AddCmd(&ishell.Cmd{
		Name:      "cp",
		Func:      commands.Cp,
		Completer: nil,
	})

	shell.AddCmd(&ishell.Cmd{
		Name:      "incp",
		Func:      commands.Incp,
		Completer: nil,
	})

	shell.AddCmd(&ishell.Cmd{
		Name:      "outcp",
		Func:      commands.Outcp,
		Completer: nil,
	})

	shell.Run()
}
