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
	"os"
	"os/exec"
	"encoding/json"
	"net/http"
	"log"
	"fmt"
	"text/template"
	"runtime"
	"strconv"
)

func SpawnBrowser(url string) {
	browser := os.Getenv("BROWSER")
	if len(browser) == 0 && runtime.GOOS == "darwin" {
		// Use default system browser on Mac OS.
		browser = "open"
	}
	if len(browser) > 0 {
		go func() {
			log.Printf("spawning browser on %s", url)
			exec.Command(browser, url).Start()
		}()
	} else {
		log.Printf("set $BROWSER to spawn browser")
	}

}

func (s *state) WritePng() {
	cmd := exec.Command("dot", "-Tpng", "-ograph.png")
	stdin, err := cmd.StdinPipe()
	check(err)
	check(cmd.Start())
	s.GraphViz(stdin)
	check(stdin.Close())
	check(cmd.Wait())
}

func loadTemplate() *template.Template {
	// This seems pretty suboptimal, but I can't figure out how else
	// to define functions before loading a template.
	return template.Must(template.New("page").Funcs(template.FuncMap{
		"kb": func(n int) string {
			return fmt.Sprintf("%dkb", n/1024)
		},
		"firstn": func(n int, xs []int) []int {
			return xs[:n]
		},
		"json": func(x interface{}) (string, error) {
			js, err := json.Marshal(x)
			return string(js), err
		},
	}).ParseFiles("page.html")).Lookup("page.html")
}

func (s *state) ServeHttp(addr string) {
	http.HandleFunc("/graph.png", func(w http.ResponseWriter, req *http.Request) {
		http.ServeFile(w, req, "graph.png")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}

		if req.Method == "POST" {
			nodeCount := 100
			if ncs := req.FormValue("nodecount"); len(ncs) > 0 {
				nc, err := strconv.ParseInt(ncs, 10, 16)
				check(err)
				nodeCount = int(nc)
			}
			s.Params = &params{
				NodeKeepCount: nodeCount,
			}
			s.WritePng()
			http.Redirect(w, req, "/", 303)
			return
		}

		// Load the template on every page load so I can edit the HTML
		// on the "live" page.
		tmpl := loadTemplate()
		err := tmpl.Execute(w, s)
		check(err)
	})

	url := addr
	if url[0] == ':' {
		url = "http://localhost" + url
	}
	SpawnBrowser(url)
	log.Fatal(http.ListenAndServe(addr, nil))
}
