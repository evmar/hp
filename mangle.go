// Copyright 2011 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"io"
	"os/exec"
)

type Demangler interface {
	Demangle(name string) (string, error)
}

type CppFilt struct {
	in  io.Writer
	out *bufio.Reader
}

func NewCppFilt() Demangler {
	cmd := exec.Command("c++filt")
	in, err := cmd.StdinPipe()
	check(err)
	out, err := cmd.StdoutPipe()
	check(err)
	cf := &CppFilt{
		in:  in,
		out: bufio.NewReader(out),
	}
	err = cmd.Start()
	check(err)
	return cf
}

func (cf *CppFilt) Demangle(name string) (string, error) {
	_, err := io.WriteString(cf.in, name+"\n")
	if err != nil {
		return "", err
	}
	res, err := mustReadLine(cf.out)
	if err != nil {
		return "", err
	}
	return string(res), nil
}
