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
	"os/exec"
	"encoding/json"
	"net/http"
	"log"
)

func (s *state) JS() []byte {
	js := make(map[string]interface{})
	js["total"] = s.profile.header.InuseBytes/1024
	js["sizes"] = s.nodeSizes
	jsbytes, err := json.Marshal(js)
	check(err)
	return jsbytes
}

func (s *state) ServeHttp(addr string) {
	http.HandleFunc("/t.png", func(w http.ResponseWriter, req *http.Request) {
		http.ServeFile(w, req, "t.png")
	})
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
		} else {
			http.ServeFile(w, req, "page.html")
		}
	})
	go func() {
		url := addr
		if url[0] == ':' {
			url = "http://localhost" + url
		}
		log.Printf("spawning browser on %s", url)
		exec.Command("gnome-open", url).Start()
	}()
	log.Fatal(http.ListenAndServe(addr, nil))
}
