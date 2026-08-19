package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-ini/ini"
	"github.com/pierrec/lz4/v4"
)

type SessionCookieJar struct {
	Cookies []Cookie `json:"cookies"`
}

type Cookie struct {
	Host             string           `json:"host"`
	Value            string           `json:"value"`
	Path             string           `json:"path"`
	Name             string           `json:"name"`
	Secure           bool             `json:"secure,omitempty"`
	Httponly         bool             `json:"httponly,omitempty"`
	OriginAttributes OriginAttributes `json:"originAttributes"`
	SameSite         int              `json:"sameSite,omitempty"`
	SchemeMap        int              `json:"schemeMap"`
	IsPartitioned    bool             `json:"isPartitioned,omitempty"`
}

type OriginAttributes struct {
	FirstPartyDomain          string `json:"firstPartyDomain"`
	GeckoViewSessionContextID string `json:"geckoViewSessionContextId"`
	PartitionKey              string `json:"partitionKey"`
	PrivateBrowsingID         int    `json:"privateBrowsingId"`
	UserContextID             int    `json:"userContextId"`
}

// GetSessionCookies returns all Firefox session cookies.
func GetSessionCookies() ([]http.Cookie, error) {
	sessionCookieJars, err := GetSessionCookieJars()
	if err != nil {
		return nil, err
	}

	var cookies []http.Cookie
	for _, sessionCookieJar := range sessionCookieJars {
		for _, cookie := range sessionCookieJar.Cookies {
			if !strings.HasSuffix(cookie.Host, "strava.com") {
				continue
			}

			//nolint:exhaustruct
			cookies = append(cookies, http.Cookie{
				Name:        cookie.Name,
				Value:       cookie.Value,
				Path:        cookie.Path,
				Domain:      cookie.Host,
				Secure:      cookie.Secure,
				HttpOnly:    cookie.Httponly,
				SameSite:    http.SameSite(cookie.SameSite),
				Partitioned: cookie.IsPartitioned,
			})

			log.Println("found session cookie:", cookie.Name)
		}
	}

	return cookies, nil
}

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
