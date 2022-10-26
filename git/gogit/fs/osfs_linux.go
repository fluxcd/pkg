//go:build linux
// +build linux

/*
Copyright 2017 Go-Git authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Copyright 2022 The Flux authors. All rights reserved.
// Adapted from: github.com/go-git/go-billy/v5/osfs

package fs

import (
	"golang.org/x/sys/unix"
)

func (f *file) Lock() error {
	f.m.Lock()
	defer f.m.Unlock()

	return unix.Flock(int(f.File.Fd()), unix.LOCK_EX)
}

func (f *file) Unlock() error {
	f.m.Lock()
	defer f.m.Unlock()

	return unix.Flock(int(f.File.Fd()), unix.LOCK_UN)
}
