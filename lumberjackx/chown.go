//go:build !linux
// +build !linux

package lumberjackx

import (
	"os"
)

func chown(_ string, _ os.FileInfo) error {
	return nil
}
