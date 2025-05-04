//go:build darwin

package main

import (
	"fmt"
	"os"
)

// FirefoxPath returns the Firefox data path.
func FirefoxPath() string {
	return fmt.Sprintf("%s/Library/Application Support/Firefox", os.Getenv("HOME"))
}
