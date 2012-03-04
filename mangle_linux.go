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
	"io"
	"fmt"
)

// This file implements a tiny subset of the C++ name mangling used on Linux,
// described at
//   http://sourcery.mentor.com/public/cxx-abi/abi.html#mangling
// Most of the complexity of name mangling is in mangling types; we only
// care about the raw function names and can ignore all that.

type stringReader struct {
	input string
	ofs   int
}

func (r *stringReader) ReadByte() (byte, error) {
	if r.ofs == len(r.input) {
		return 0, io.EOF
	}
	r.ofs++
	return r.input[r.ofs-1], nil
}
func (r *stringReader) UnreadByte() {
	if r.ofs == 0 {
		panic("unread past beginning")
	}
	r.ofs--
}

func (r *stringReader) ReadN(n int) (string, error) {
	str := r.input[r.ofs:r.ofs+n]
	r.ofs += len(str)
	if len(str) < n {
		return str, io.EOF
	}
	return str, nil
}

type mr struct {
	*stringReader
	output   string
	leftover string
}

func (r *mr) Write(str string) {
	r.output += str
}

func (r *mr) ReadSourceName() (string, error) {
	count := 0
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if '0' <= c && c <= '9' {
			count = count * 10 + int(c - '0')
		} else {
			r.UnreadByte()
			break
		}
	}

	buf, err := r.ReadN(count)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

func CVQualifier(c byte) string {
	switch c {
	case 'r': return "restrict"
	case 'V': return "volatile"
	case 'K': return "const"
	}
	return ""
}

func (r *mr) ReadSubstitution() (string, error) {
	// XXX this special-cases out just the cases we've hit.

	b, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	switch b {
	case 't':
		return "std", nil
	default:
		for '0' <= b && b <= '9' {
			b, err = r.ReadByte()
			if err != nil {
				return "", err
			}
		}
	}
	return "XXX", nil
}

func (r *mr) ReadTemplateArgs() error {
	// Hacky attempt to skim past template args.
	depth := 1
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil
		}
		switch b {
		case 'E':
			depth--
			if depth == 0 {
				return nil
			}
		case 'X': // expression
			depth++
		case 'J': // argument pack
			depth++
		case 'L': // expr-primary
			depth++
		case 'I': // nested template
			depth++
		case 'N': // nested name
			depth++
		case 'S':
			// Specially handle substitutions because "S12_" would otherwise
			// confuse the default branch below.
			_, err = r.ReadSubstitution()
			if err != nil {
				return err
			}
		default:
			if '0' <= b && b <= '9' {
				r.UnreadByte()
				_, err = r.ReadSourceName()
				if err != nil {
					return err
				}
			} else {
				// hope for the best!
			}
		}
	}
	panic("not reached")
}

func (r *mr) ReadNestedName() (string, error) {
	fullname := ""
	name := ""
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		switch {
		case '0' <= b && b <= '9':
			r.UnreadByte()
			name, err = r.ReadSourceName()
			if err != nil {
				return "", err
			}
			if fullname != "" {
				fullname += "::"
			}
			fullname += name
		case b == 'E':  // done
			return fullname, nil
		case b == 'C':  // ctor
			b, err := r.ReadByte()
			if err != nil {
				return "", err
			}
			switch b {
			case '1', '2', '3': // see spec
				fullname += "::" + name
			default:
				r.UnreadByte()
				return "", fmt.Errorf("unexpected '%c' reading ctor type", b)
			}
		case b == 'S':  // St => ::std::
			sub, err := r.ReadSubstitution()
			if err != nil {
				return "", err
			}
			fullname += sub
		case b == 'I':  // template-args
			err := r.ReadTemplateArgs()
			if err != nil {
				return "", err
			}
			fullname += "<>"
		case len(CVQualifier(b)) > 0:
			// Ignore cv-qualifier for now.
			continue
		default:
			r.UnreadByte()
			return "", fmt.Errorf("unexpected '%c' reading nested name", b)
		}
	}
	panic("not reached")
}

func (r *mr) demangle(name string) error {
	header, err := r.ReadN(2)
	if err != nil {
		return err
	}
	if header != "_Z" {
		// Nothing to demangle.
		r.output = name
		return nil
	}

	b, err := r.ReadByte()
	if err != nil {
		return err
	}
	switch b {
	case 'N': // nested-name
		fullname, err := r.ReadNestedName()
		if err != nil {
			return err
		}
		r.Write(fullname)
	default:
		if '0' <= b && b <= '9' {
			name, err := r.ReadSourceName()
			if err != nil {
				return err
			}
			r.Write(name)
		}
	}

	if len(r.output) == 0 {
		return fmt.Errorf("didn't produce any demangled text")
	}

	if r.stringReader.ofs < len(r.stringReader.input) {
		r.leftover = r.stringReader.input[r.stringReader.ofs:]
	}

	return nil
}

type LinuxDemangler bool
func NewLinuxDemangler(includeLeftover bool) *LinuxDemangler {
	l := LinuxDemangler(includeLeftover)
	return &l
}
func (d *LinuxDemangler) Demangle(name string) (string, error) {
	mr := &mr{
	stringReader: &stringReader{name, 0},
	}
	err := mr.demangle(name)
	if err != nil {
		return "", fmt.Errorf("demangling '%s' near '%s': %s", name, mr.stringReader.input[mr.stringReader.ofs:], err)
	}

	out := mr.output
	if bool(*d) && len(mr.leftover) > 0 {
		out += fmt.Sprintf(" (leftover %s)", mr.leftover)
	}
	return out, nil
}
