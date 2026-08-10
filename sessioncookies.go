package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-ini/ini"
	"github.com/pierrec/lz4/v4"
)

// GetSessionCookieJars extracts all session cookies from all Firefox profiles.
func GetSessionCookieJars() ([]*SessionCookieJar, error) {
	iniPath := fmt.Sprintf("%s/profiles.ini", FirefoxPath())

	ini, err := ini.Load(iniPath)
	if err != nil {
		return nil, err
	}

	var profilePaths []string

	for _, section := range ini.Sections() {
		if strings.HasPrefix(section.Name(), "Profile") {
			key, err := section.GetKey("Path")
			if err != nil {
				return nil, err
			}

			profilePaths = append(profilePaths, key.String())
		}
	}

	var jars []*SessionCookieJar

	for _, profilePath := range profilePaths {
		filename := fmt.Sprintf("%s/%s/sessionstore-backups/recovery.jsonlz4", FirefoxPath(), profilePath)

		log.Println("extracting cookies from", filename)

		jar, err := ExtractLZ4(filename)
		if err != nil {
			return nil, err
		}

		jars = append(jars, jar)
	}

	return jars, nil
}

// ExtractLZ4 takes a filename and reads the compressed content into a SessionCookieJar.
func ExtractLZ4(filename string) (*SessionCookieJar, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	if len(data) < 12 {
		return nil, errors.New("lz4 file too short")
	}

	// get uncompressed size (little endian uint32 at offset 8)
	outSize := binary.LittleEndian.Uint32(data[8:12])
	dst := make([]byte, outSize)

	// decompress from offset 12
	src := data[12:]
	n, err := lz4.UncompressBlock(src, dst)
	if err != nil {
		return nil, err
	}

	if uint32(n) != outSize {
		return nil, fmt.Errorf("unexpected decompressed size: got %d, want %d", n, outSize)
	}

	var sessionCookieJar SessionCookieJar
	if err := json.Unmarshal(dst, &sessionCookieJar); err != nil {
		return nil, err
	}

	return &sessionCookieJar, nil
}
