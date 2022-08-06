//go:build disable_mega
// +build disable_mega

package engine

import "fmt"

func NewMegaDownload(link string, listener *MirrorListener) error {
	return fmt.Errorf("Mega download isnt supported in this build")
}
