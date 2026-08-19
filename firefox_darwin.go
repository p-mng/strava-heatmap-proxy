package main

import (
	"fmt"
	"os"
)

// FirefoxPath returns the Firefox data path on macOS.
func FirefoxPath() string {
	return fmt.Sprintf("%s/Library/Application Support/Firefox", os.Getenv("HOME"))
}
