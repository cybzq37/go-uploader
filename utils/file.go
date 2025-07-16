package utils

import (
	"os"
	"path/filepath"
)

func EnsureDir(path string) {
	os.MkdirAll(filepath.Dir(path), os.ModePerm)
}
