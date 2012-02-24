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
	"sort"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type sortableSlice struct {
	xs  []interface{}
	key func(interface{}) int
}

func (s sortableSlice) Len() int           { return len(s.xs) }
func (s sortableSlice) Swap(i, j int)      { s.xs[i], s.xs[j] = s.xs[j], s.xs[i] }
func (s sortableSlice) Less(i, j int) bool { return s.key(s.xs[i]) < s.key(s.xs[j]) }

func Sort(xs []interface{}, key func(interface{}) int) {
	sort.Sort(sortableSlice{xs, key})
}
