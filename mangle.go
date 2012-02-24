package main

import (
	"bufio"
	"io"
	"os/exec"
)

type Demangler struct {
	in  io.Writer
	out *bufio.Reader
}

func NewDemangler() *Demangler {
	cmd := exec.Command("c++filt")
	in, err := cmd.StdinPipe()
	check(err)
	out, err := cmd.StdoutPipe()
	check(err)
	d := &Demangler{
		in:  in,
		out: bufio.NewReader(out),
	}
	err = cmd.Start()
	check(err)
	return d
}

func (d *Demangler) Demangle(name string) string {
	_, err := io.WriteString(d.in, name+"\n")
	check(err)
	res, err := mustReadLine(d.out)
	check(err)
	return string(res)
}
