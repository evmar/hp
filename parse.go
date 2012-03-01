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
	"bytes"
	"io"
	"log"
	"regexp"
	"sort"
	"strconv"
)

type Stats struct {
	InuseObjects, InuseBytes, AllocObjects, AllocBytes int
}

func (s *Stats) Add(other *Stats) {
	s.InuseObjects += other.InuseObjects
	s.InuseBytes += other.InuseBytes
	s.AllocObjects += other.AllocObjects
	s.AllocBytes += other.AllocBytes
}

type Stack struct {
	Stats *Stats
	Stack []uint64
}

type MapEntry struct {
	start, end uint64
	path       string
}

type Maps []*MapEntry

func (m Maps) Search(addr uint64) *MapEntry {
	i := sort.Search(len(m), func(i int) bool {
		return m[i].end > addr
	})
	if i < len(m) {
		e := m[i]
		if e.start <= addr {
			return e
		}
	}
	return nil
}

type Profile struct {
	Header *Stats
	stacks []*Stack
	maps   Maps
}

func mustReadLine(r *bufio.Reader) ([]byte, error) {
	line, prefix, err := r.ReadLine()
	if prefix {
		panic("prefix")
	}
	return line, err
}

func parseAddr(str []byte) uint64 {
	x, err := strconv.ParseUint(string(str), 16, 64)
	check(err)
	return x
}

var re_stats *regexp.Regexp = regexp.MustCompile(`^\s+(\d+):\s+(\d+) \[\s*(\d+):\s+(\d+)\] @ ?(.*)`)

func parseStats(line []byte) (*Stats, []byte) {
	match := re_stats.FindSubmatch(line)
	if match == nil || len(match) != 6 {
		panic("bad stats line '" + string(line) + "'")
	}
	var ints [4]int
	for i := 0; i < 4; i++ {
		x, err := strconv.ParseUint(string(match[i+1]), 10, 32)
		check(err)
		ints[i] = int(x)
	}
	s := &Stats{ints[0], ints[1], ints[2], ints[3]}
	return s, match[5]
}

func ParseHeap(r *bufio.Reader) *Profile {
	line, err := mustReadLine(r)
	check(err)

	headerPrefix := []byte("heap profile:")
	if !bytes.HasPrefix(line, headerPrefix) {
		panic("bad header" + string(line))
	}
	line = line[len(headerPrefix):]

	profile := &Profile{}

	header, _ := parseStats(line)
	profile.Header = header

	mapped_section := []byte("MAPPED_LIBRARIES:")
	for {
		line, err := mustReadLine(r)
		check(err)

		if bytes.Equal(line, mapped_section) {
			break
		}
		if len(line) == 0 {
			continue
		}
		stats, rest := parseStats(line)

		// XXX filter here
		if stats.InuseBytes == 0 {
			continue
		}

		if len(rest) == 0 {
			log.Printf("warning: no stacks on %q", line)
			continue
		}

		stackStrs := bytes.Split(rest, []byte(" "))
		stack := make([]uint64, 0, len(stackStrs))
		for _, str := range stackStrs {
			if !bytes.HasPrefix(str, []byte("0x")) {
				panic("non hex address? '" + string(rest) + "'")
			}
			addr := parseAddr(str[2:])
			stack = append(stack, addr)
		}
		profile.stacks = append(profile.stacks, &Stack{Stats: stats, Stack: stack})
	}

	re_map := regexp.MustCompile(`^([0-9a-f]+)-([0-9a-f]+) (....) ([0-9a-f]+) ..... (\d+)\s*(.*)`)
	for {
		line, err := mustReadLine(r)
		if err == io.EOF {
			break
		}
		check(err)

		match := re_map.FindSubmatch(line)
		if match == nil || len(match) != 7 {
			panic("bad maps line " + string(line))
		}
		start := parseAddr(match[1])
		end := parseAddr(match[2])
		file := string(match[6])

		entry := &MapEntry{start, end, file}
		profile.maps = append(profile.maps, entry)
	}

	return profile
}
