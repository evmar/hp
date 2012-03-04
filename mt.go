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

// This file tests mangle_linux.go, and can be removed or renamed once
// it's in better shape.

package main

import (
	"fmt"
	"regexp"
	"bufio"
	"os"
	"io"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func test() {
	syms := []string{
		"_ZN3net23TCPClientSocketLibevent14DoReadCallbackEi",
		"_ZN14ProfileManager22DoFinalInitForServicesEP7Profileb",
		"_ZNK3gfx17PlatformFontPango10DeriveFontEii",
		"_ZN10extensions16SettingsFrontendC2ERK13scoped_refptrINS_22SettingsStorageFactoryEEP7Profile",
		"_ZNSt8_Rb_treeISsSt4pairIKSsPN4base5ValueEESt10_Select1stIS5_ESt4lessISsESaIS5_EE16_M_insert_uniqueERKS5_",
	}
	d := NewLinuxDemangler()
	for _, sym := range syms {
		fmt.Printf("%s\n", sym)
		dem, err := d.Demangle(sym)
		check(err)
		fmt.Printf("=> %s\n", dem)
	}
}

func output(buf []byte) {
	_, err := os.Stdout.Write(buf)
	if err != nil {
		panic(err)
	}
}

func filt() {
	d := NewLinuxDemangler()
	re := regexp.MustCompile(`_Z[_a-zA-Z0-9]+`)
	r := bufio.NewReader(os.Stdin)
	for {
		line, isPrefix, err := r.ReadLine()
		if isPrefix {
			panic("overlong line")
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		offsets := re.FindAllIndex(line, -1)
		if offsets != nil {
			fmt.Printf("%s\n", line)
			last := 0
			for _, match := range offsets {
				start, end := match[0], match[1]
				output(line[last:start])
				dem, err := d.Demangle(string(line[start:end]))
				check(err)
				output([]byte(dem))
				last = end
			}
			output(line[last:])
			output([]byte("\n"))
			output([]byte("\n"))
		}
	}
}

func main() {
	test()
}
