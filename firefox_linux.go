//go:build linux

package main

import (
	"fmt"
	"os"
)

// FirefoxPath returns the Firefox data path.
func FirefoxPath() string {
	return fmt.Sprintf("%s/.mozilla/firefox", os.Getenv("HOME"))
}
