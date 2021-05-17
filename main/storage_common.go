package main

import (
	"os"
	"path/filepath"
)

func dirSize(path string) (uint64, error) {
	var size int64
	err := filepath.Walk(path,
		func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				size += info.Size()
			}
			return nil
		})
	return uint64(size), err
}
