// Package util holds formatting and helper functions shared across the bot.
//
// The formatting helpers (GetHumanBytes, GetProgressBarString, CalculateETA,
// HumanizeDuration) reproduce the original MirrorBotGo output byte-for-byte;
// the status display depends on it.
package util

import (
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ProgressMaxSize is the number of full-block cells a 100% bar uses (100/8).
const ProgressMaxSize = 100 / 8

// ProgressIncomplete is the 8-step ramp of partial block glyphs.
var ProgressIncomplete = []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}

const (
	magnetRegex    = `magnet:\?xt=urn:btih:[a-zA-Z0-9]*`
	urlRegex       = `(?:(?:https?|ftp):\/\/)?[\w/\-?=%.]+\.[\w/\-?=%.]+`
	driveLinkRegex = `https://drive\.google\.com/(drive)?/?u?/?\d?/?(mobile)?/?(file)?(folders)?/?d?/([-\w]+)[?+]?/?(w+)?`
)

var (
	reMagnet = regexp.MustCompile(magnetRegex)
	reURL    = regexp.MustCompile(urlRegex)
	reDrive  = regexp.MustCompile(driveLinkRegex)
)

// GetHumanBytes formats a byte count using 1024-based units, matching the
// original output exactly: "0 B", "1.0 kB", "12.5 MB", etc.
func GetHumanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

// GetProgressBarString renders a fixed-width unicode progress bar wrapped in
// square brackets, identical to the original implementation.
func GetProgressBarString(current, total int) string {
	var p int
	if total == 0 {
		p = 0
	} else {
		p = current * 100 / total
	}
	p = int(math.Min(math.Max(float64(p), 0), 100))
	cFull := p / 8
	cPart := p%8 - 1
	var out string
	out += strings.Repeat("█", cFull)
	if cPart >= 0 {
		out += ProgressIncomplete[cPart]
	}
	out += strings.Repeat(" ", ProgressMaxSize-cFull)
	return fmt.Sprintf("[%s]", out)
}

// CalculateETA returns the estimated time remaining, smart-rounded to friendly
// intervals, matching the original.
func CalculateETA(bytesLeft, speed int64) time.Duration {
	if speed == 0 {
		return time.Duration(0)
	}
	eta := time.Duration(bytesLeft/speed) * time.Second
	switch {
	case eta > 8*time.Hour:
		eta = eta.Round(time.Hour)
	case eta > 4*time.Hour:
		eta = eta.Round(30 * time.Minute)
	case eta > 2*time.Hour:
		eta = eta.Round(15 * time.Minute)
	case eta > time.Hour:
		eta = eta.Round(5 * time.Minute)
	case eta > 30*time.Minute:
		eta = eta.Round(1 * time.Minute)
	case eta > 15*time.Minute:
		eta = eta.Round(30 * time.Second)
	case eta > 5*time.Minute:
		eta = eta.Round(15 * time.Second)
	case eta > time.Minute:
		eta = eta.Round(5 * time.Second)
	}
	return eta
}

// HumanizeDuration renders a duration as "1d 2h 3m 4s" style, matching the original.
func HumanizeDuration(duration time.Duration) string {
	if duration.Seconds() < 60.0 {
		return fmt.Sprintf("%ds", int64(duration.Seconds()))
	}
	if duration.Minutes() < 60.0 {
		remainingSeconds := math.Mod(duration.Seconds(), 60)
		return fmt.Sprintf("%dm %ds", int64(duration.Minutes()), int64(remainingSeconds))
	}
	if duration.Hours() < 24.0 {
		remainingMinutes := math.Mod(duration.Minutes(), 60)
		remainingSeconds := math.Mod(duration.Seconds(), 60)
		return fmt.Sprintf("%dh %dm %ds",
			int64(duration.Hours()), int64(remainingMinutes), int64(remainingSeconds))
	}
	remainingHours := math.Mod(duration.Hours(), 24)
	remainingMinutes := math.Mod(duration.Minutes(), 60)
	remainingSeconds := math.Mod(duration.Seconds(), 60)
	return fmt.Sprintf("%dd %dh %dm %ds",
		int64(duration.Hours()/24), int64(remainingHours),
		int64(remainingMinutes), int64(remainingSeconds))
}

// ParseMessageArgs returns the text after the first space (the command argument).
func ParseMessageArgs(m string) string {
	args := strings.SplitN(m, " ", 2)
	if len(args) >= 2 {
		return strings.TrimSpace(args[1])
	}
	return ""
}

// IsMagnetLink reports whether the text contains a magnet URI.
func IsMagnetLink(link string) bool { return reMagnet.MatchString(link) }

// IsURLLink reports whether the text looks like a URL.
func IsURLLink(link string) bool { return reURL.MatchString(link) }

// GetFileIDByGDriveLink extracts a Google Drive file/folder id from a share URL.
func GetFileIDByGDriveLink(link string) string {
	if !strings.Contains(link, "https://drive.google.com") {
		return ""
	}
	if id := gdriveIDFromParams(link); id != "" {
		return id
	}
	matches := reDrive.FindStringSubmatch(link)
	if len(matches) >= 2 {
		return matches[len(matches)-2]
	}
	return ""
}

func gdriveIDFromParams(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	if v := u.Query()["id"]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// IsPathDir reports whether path exists and is a directory.
func IsPathDir(pth string) bool {
	fi, err := os.Stat(pth)
	if err != nil {
		return false
	}
	return fi.Mode().IsDir()
}

// IsPathExists reports whether path exists.
func IsPathExists(pth string) bool {
	_, err := os.Stat(pth)
	return !os.IsNotExist(err)
}

// RemoveByPath removes a path recursively.
func RemoveByPath(pth string) error { return os.RemoveAll(pth) }

// GetFileBaseName returns the last path segment.
func GetFileBaseName(path string) string { return filepath.Base(path) }

// GetFileBaseNameNoExt returns the last path segment without its extension.
func GetFileBaseNameNoExt(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

var gidRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandGID returns a random alphabetic identifier of length n (used for transfer gids).
func RandGID(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = gidRunes[rand.Intn(len(gidRunes))]
	}
	return string(b)
}

// ParseInt64 parses a base-10 int64, returning 0 on error.
func ParseInt64(s string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v
}
