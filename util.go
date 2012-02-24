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
