package testutils

import (
	"io/ioutil"
	"os"
	"path"
)

type TempDir struct {
	Path string
}

func NewTempDir() (*TempDir, error) {
	return NewTempDirWith("", "")
}

func NewTempDirWith(dir string, prefix string) (*TempDir, error) {
	tempPath, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		return nil, err
	}
	return &TempDir{Path: tempPath}, nil
}

func (t *TempDir) Remove() error {
	return os.RemoveAll(t.Path)
}

func (t *TempDir) InPath(joins ...string) string {
	allJoins := []string{
		t.Path,
	}
	allJoins = append(allJoins, joins...)
	return path.Join(allJoins...)
}
