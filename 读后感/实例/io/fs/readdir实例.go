package main

import "io/fs"

type readDirOnly struct{ fs.ReadDirFS }

func (readDirOnly) Open(name string) (fs.File, error) { return nil, fs.ErrNotExist }

func main() {
	dirs, err := fs.ReadDir(readDirOnly{testFsys}, ".")
}
go ReadDirFS