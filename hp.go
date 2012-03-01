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
	"io"
	"sort"
)

var flag_http *string = flag.String("http", "", "http service address (e.g. ':8000')")
var flag_profile *bool = flag.Bool("profile", false, "whether to profile hp itself")
var flag_syms *string = flag.String("syms", "", "load symbols from file instead of binary")

type state struct {
	Profile   *Profile
	demangler *Demangler
	Graph     *graph
}

type Node struct {
	addr     uint64
	name     string
	cur, cum Stats
}
type edge struct {
	src, dst *Node
}
type graph struct {
	nodes map[uint64]*Node
	NodeSizes []int
	edges map[edge]int
}

func CleanupStacks(stacks []*Stack, syms Symbols) map[uint64]string {
	// Map of symbol name -> address for that symbol.
	addrs := make(map[string]uint64)
	// Same map, in reverse.
	names := make(map[uint64]string)

	for _, stack := range stacks {
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

func (s *state) Label(n *Node) string {
	label := n.name

	if len(label) == 0 {
		label = fmt.Sprintf("0x%x", n.addr)
		e := s.Profile.maps.Search(n.addr)
		if e != nil {
			label += fmt.Sprintf(" [%s]", e.path)
		}
	} else {
		label = s.demangler.Demangle(label)
		label = RemoveTypes(label)
	}
	return label
}

func (s *state) SizeLabel(n *Node) string {
	cur := n.cur.InuseBytes
	cum := n.cum.InuseBytes
	frac := float32(cum) / float32(s.Profile.Header.InuseBytes)
	return fmt.Sprintf("%dk of %dk (%.1f%% of total)", cur/1024, cum/1024, frac * 100.0)
}

func (g *graph) Analyze(stacks []*Stack, names map[uint64]string) {
	// Accumulate stats into nodes and edges.
	for _, stack := range stacks {
		var last *Node
		for _, addr := range stack.Stack {
			if last != nil && addr == last.addr {
				continue // Ignore loops
			}

			node := g.nodes[addr]
			if node == nil {
				node = &Node{addr: addr, name: names[addr]}
				g.nodes[addr] = node
			}

			if last == nil {
				node.cur.Add(stack.Stats)
			} else {
				g.edges[edge{node, last}] += stack.Stats.InuseBytes
			}
			node.cum.Add(stack.Stats)

			last = node
		}
	}

	// Collect node sizes.
	nodeSizes := make([]int, 0, len(g.nodes))
	for _, n := range g.nodes {
		size := n.cum.InuseBytes
		if size > 0 {
			nodeSizes = append(nodeSizes, size)
		}
	}

	sort.Ints(nodeSizes)

	// Reverse to descending order.
	for i := 0; i < len(nodeSizes)/2; i++ {
		j := len(nodeSizes)-i-1
		nodeSizes[i], nodeSizes[j] = nodeSizes[j], nodeSizes[i]
	}

	g.NodeSizes = nodeSizes
}

func (s *state) GraphViz(w io.Writer) {
	g := s.Graph

	fmt.Fprintf(w, "digraph G {\n")
	fmt.Fprintf(w, "nodesep = 0.2\n")
	fmt.Fprintf(w, "ranksep = 0.3\n")
	fmt.Fprintf(w, "node [fontsize=9]\n")
	fmt.Fprintf(w, "edge [fontsize=8]\n")

	// Select top N nodes.
	keptNodes := make(map[*Node]bool)
	nodeKeepCount := 100
	nodeKeepThreshold := g.NodeSizes[nodeKeepCount]
	log.Printf("keeping %d nodes with cumulative >= %dk", nodeKeepCount, nodeKeepThreshold/1024)
	for _, n := range g.nodes {
		if n.cum.InuseBytes >= nodeKeepThreshold {
			keptNodes[n] = true
		}
	}

	// Order edges that reference selected nodes by size.
	edgelist := make([]interface{}, 0, len(g.edges))
	for e, _ := range g.edges {
		if keptNodes[e.src] && keptNodes[e.dst] {
			edgelist = append(edgelist, e)
		}
	}
	Sort(edgelist, func(e interface{}) int { return -g.edges[e.(edge)] })

	indegree := make(map[*Node]int)
	outdegree := make(map[*Node]int)
	for _, e := range edgelist {
		edge := e.(edge)
		size := g.edges[edge]

		if indegree[edge.dst] == 0 {
			// Keep at least one edge for each dest.
		} else if size/1024 < 30 {
			continue
		}
		outdegree[edge.src]++
		indegree[edge.dst]++
		fmt.Fprintf(w, "%d -> %d [label=\" %.1f\"]\n", edge.src.addr, edge.dst.addr, float32(g.edges[edge])/1024.0)
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
		label := s.Label(n) + "\\n" + s.SizeLabel(n)
		fmt.Fprintf(w, "%d [label=\"%s\",shape=box,href=\"%d\"]\n", n.addr, label, n.addr)
	}
	log.Printf("total not shown: %.1fk", float32(missing)/1024.0)
	log.Printf("total kept nodes: %.1fk", float32(total)/1024.0)

	fmt.Fprintf(w, "}\n")
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

	if len(profilePath) == 0 {
		log.Fatalf("usage: %s binary profile", os.Args[0])
	}

	noLoad := false

	profChan := make(chan *Profile)
	go func() {
		if noLoad {
			profChan <- nil
			return
		}
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
			if noLoad {
				symChan <- nil
				return
			}
			log.Printf("reading symbol map from %s", symsPath)
			syms := LoadSymsMap(symsPath)
			log.Printf("loaded %d syms", len(syms))
			symChan <- syms
		}()
	}

	syms := <-symChan
	profile := <-profChan

	state := &state{
		Profile: profile,
		demangler: NewDemangler(),
	}
	var names map[uint64]string
	if noLoad {
		syms = syms
	} else {
		names = CleanupStacks(state.Profile.stacks, syms)
	}

	state.Graph = &graph{
		nodes: make(map[uint64]*Node),
		edges: make(map[edge]int),
	}
	state.Graph.Analyze(profile.stacks, names)

	if len(*flag_http) > 0 {
		log.Printf("serving on %s", *flag_http)
		state.ServeHttp(*flag_http)
	} else {
		log.Printf("writing output...")
		state.GraphViz(os.Stdout)
	}

	log.Printf("done")
}
