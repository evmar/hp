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
	"debug/elf"
	"regexp"
	"sort"
	"strings"
	"os"
	"bufio"
	"io"
	"strconv"
)

type Symbol struct {
	addr, size uint64
	name       string
}
type Symbols []*Symbol

func (s Symbols) Len() int           { return len(s) }
func (s Symbols) Less(i, j int) bool { return s[i].addr < s[j].addr }
func (s Symbols) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func stripDotted(name string) string {
	// Sometimes symbols have suffixes like ".part.123" or ".isra.134".
	// Strip it.
	ofs := strings.Index(name, ".")
	if ofs >= 0 {
		return name[:ofs]
	}
	return name
}

func LoadSyms(path string) Symbols {
	f, err := elf.Open(path)
	check(err)
	elfsyms, err := f.Symbols()
	check(err)
	f.Close()

	syms := make(Symbols, 0, len(elfsyms))
	for _, sym := range elfsyms {
		if sym.Value > 0 && sym.Size > 0 {
			syms = append(syms, &Symbol{addr: sym.Value, size: sym.Size, name: stripDotted(sym.Name)})
		}
	}
	sort.Sort(syms)
	return syms
}

func LoadSymsMap(path string) Symbols {
	f, err := os.Open(path)
	check(err)
	r := bufio.NewReader(f)

	var syms Symbols
	for {
		s, err := r.ReadSlice(' ')
		if err == io.EOF {
			break
		}
		check(err)
		addr := parseAddr(s[:len(s)-1])

		s, err = r.ReadSlice(' ')
		check(err)
		size, err := strconv.ParseUint(string(s[:len(s)-1]), 10, 64)
		check(err)

		name, err := r.ReadSlice('\n')
		check(err)

		syms = append(syms, &Symbol{addr, size, stripDotted(string(name[:len(name)-1]))})
	}
	return syms
}

func (syms Symbols) Lookup(addr uint64) *Symbol {
	i := sort.Search(len(syms), func(i int) bool {
		return syms[i].addr > addr
	})
	if i < len(syms) && i > 0 {
		sym := syms[i-1]
		if sym.addr <= addr && sym.addr+uint64(sym.size) > addr {
			return sym
		}
	}
	return nil
}

func replaceAll(re *regexp.Regexp, str string) string {
	for {
		newstr := re.ReplaceAllString(str, "")
		if newstr == str {
			return str
		}
		str = newstr
	}
	return str
}

var paren_re *regexp.Regexp = regexp.MustCompile(`\([^()]*\)`)
var template_re *regexp.Regexp = regexp.MustCompile(`<[^<>]*>`)

func RemoveTypes(name string) string {
	name = replaceAll(paren_re, name)
	name = replaceAll(template_re, name)
	return name
}
