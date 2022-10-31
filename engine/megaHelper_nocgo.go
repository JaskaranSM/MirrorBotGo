//go:build !enable_mega
// +build !enable_mega

package engine

import "fmt"

func NewMegaDownload(link string, listener *MirrorListener) error {
	return fmt.Errorf("Mega download isnt supported in this build")
}
