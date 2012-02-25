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
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
)

var flag_profile *bool = flag.Bool("profile", false, "whether to profile hp itself")
var flag_syms *string = flag.String("syms", "", "load symbols from file instead of binary")

func CleanupStacks(profile *Profile, syms Symbols) map[uint64]string {
	// Map of symbol name -> address for that symbol.
	addrs := make(map[string]uint64)
	// Same map, in reverse.
	names := make(map[uint64]string)

	for _, stack := range profile.stacks {
		var last uint64
		var newstack []uint64
		for _, addr := range stack.Stack {

			// Map address to symbol, then symbol back to a canonical
			// address.  This means multiple points within the same
			// function end up as a single node.
			sym := syms.Lookup(addr)
			if sym != nil {
				name := sym.name

				new_addr, known := addrs[name]
				if known {
					addr = new_addr
				} else {
					addrs[name] = addr
				}

				names[addr] = name
			}

			if addr == last {
				continue
			}
			newstack = append(newstack, addr)
			last = addr
		}
		stack.Stack = newstack
	}
	return names
}

type Node struct {
	addr     uint64
	name     string
	cur, cum Stats
}

func Label(p *Profile, n *Node, d *Demangler) string {
	label := n.name

	if len(label) == 0 {
		label = fmt.Sprintf("0x%x", n.addr)
		e := p.maps.Search(n.addr)
		if e != nil {
			label += fmt.Sprintf(" [%s]", e.path)
		}
	} else {
		label = d.Demangle(label)
		label = RemoveTypes(label)
	}
	return label
}

func SizeLabel(total, cur, cum int) string {
	return fmt.Sprintf("%dk of %dk (%.1f%% of total)", cur/1024, cum/1024,
		float32(cum)*100.0/float32(total))
}

func GraphViz(p *Profile, names map[uint64]string, d *Demangler) {
	type edge struct {
		src, dst *Node
	}
	nodes := make(map[uint64]*Node)
	edges := make(map[edge]int)

	// Accumulate stats into nodes and edges.
	for _, stack := range p.stacks {
		var last *Node
		for _, addr := range stack.Stack {
			if last != nil && addr == last.addr {
				continue // Ignore loops
			}

			node := nodes[addr]
			if node == nil {
				node = &Node{addr: addr, name: names[addr]}
				nodes[addr] = node
			}

			if last == nil {
				node.cur.Add(stack.Stats)
			} else {
				edges[edge{node, last}] += stack.Stats.InuseBytes
			}
			node.cum.Add(stack.Stats)

			last = node
		}
	}

	// Order nodes by size.
	nodelist := make([]interface{}, 0, len(nodes))
	for _, n := range nodes {
		nodelist = append(nodelist, n)
	}
	Sort(nodelist, func(n interface{}) int { return -n.(*Node).cum.InuseBytes })

	fmt.Printf("digraph G {\n")
	fmt.Printf("nodesep = 0.2\n")
	fmt.Printf("ranksep = 0.3\n")
	fmt.Printf("node [fontsize=9]\n")
	fmt.Printf("edge [fontsize=8]\n")

	// Select top N nodes.
	keptNodes := make(map[*Node]bool)
	nodeKeepCount := 100
	log.Printf("keeping nodes with cumulative >= %.1fk", float32(nodelist[nodeKeepCount].(*Node).cum.InuseBytes)/1024.0)
	for _, xn := range nodelist[:nodeKeepCount] {
		n := xn.(*Node)
		keptNodes[n] = true
	}

	// Order edges that reference selected nodes by size.
	edgelist := make([]interface{}, 0, len(edges))
	for e, _ := range edges {
		if keptNodes[e.src] && keptNodes[e.dst] {
			edgelist = append(edgelist, e)
		}
	}
	Sort(edgelist, func(e interface{}) int { return -edges[e.(edge)] })

	indegree := make(map[*Node]int)
	outdegree := make(map[*Node]int)
	for _, e := range edgelist {
		edge := e.(edge)
		size := edges[edge]

		if indegree[edge.dst] == 0 {
			// Keep at least one edge for each dest.
		} else if size/1024 < 30 {
			continue
		}
		outdegree[edge.src]++
		indegree[edge.dst]++
		fmt.Printf("%d -> %d [label=\" %.1f\"]\n", edge.src.addr, edge.dst.addr, float32(edges[edge])/1024.0)
	}

	total := 0
	missing := 0
	for n, _ := range keptNodes {
		if indegree[n] == 0 && outdegree[n] == 0 {
			log.Printf("no edges for %x (%.1fk)", n.addr, float32(n.cum.InuseBytes)/1024.0)
			missing += n.cum.InuseBytes
			continue
		}
		total += n.cur.InuseBytes
		label := Label(p, n, d) + "\\n" + SizeLabel(p.header.InuseBytes, n.cur.InuseBytes, n.cum.InuseBytes)
		fmt.Printf("%d [label=\"%s\",shape=box,href=\"%d\"]\n", n.addr, label, n.addr)
	}
	log.Printf("total not shown: %.1fk", float32(missing)/1024.0)
	log.Printf("total kept nodes: %.1fk", float32(total)/1024.0)

	fmt.Printf("}\n")
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] binary profile\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *flag_profile {
		f, err := os.Create("goprof")
		check(err)
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	var symsPath, binaryPath, profilePath string
	if len(*flag_syms) > 0 {
		symsPath, profilePath = *flag_syms, flag.Arg(0)
	} else {
		binaryPath, profilePath = flag.Arg(0), flag.Arg(1)
	}
	log.Printf("%q %q %q", symsPath, binaryPath, profilePath)

	if len(profilePath) == 0 {
		log.Fatalf("usage: %s binary profile", os.Args[0])
	}

	profChan := make(chan *Profile)
	go func() {
		log.Printf("reading profile from %s", profilePath)
		f, err := os.Open(profilePath)
		check(err)
		profile := ParseHeap(bufio.NewReader(f))
		f.Close()
		log.Printf("loaded %d stacks", len(profile.stacks))
		profChan <- profile
	}()

	symChan := make(chan Symbols)
	if len(binaryPath) > 0 {
		go func() {
			log.Printf("reading symbols from %s", binaryPath)
			syms := LoadSyms(binaryPath)
			log.Printf("loaded %d syms", len(syms))
			symChan <- syms
		}()
	} else {
		go func() {
			log.Printf("reading symbol map from %s", symsPath)
			syms := LoadSymsMap(symsPath)
			log.Printf("loaded %d syms", len(syms))
			symChan <- syms
		}()
	}

	syms := <-symChan
	profile := <-profChan

	demangler := NewDemangler()

	log.Printf("writing output...")

	names := CleanupStacks(profile, syms)
	GraphViz(profile, names, demangler)

	log.Printf("done")
}
