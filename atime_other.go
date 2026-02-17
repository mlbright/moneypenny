//go:build !linux

package main

import (
	"os"
	"time"
)

func atime(info os.FileInfo) time.Time {
	return info.ModTime()
}
