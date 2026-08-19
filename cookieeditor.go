package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type ExportedCookies []struct {
	Name             string  `json:"name"`
	Value            string  `json:"value"`
	Domain           string  `json:"domain"`
	HostOnly         bool    `json:"hostOnly"`
	Path             string  `json:"path"`
	Secure           bool    `json:"secure"`
	HTTPOnly         bool    `json:"httpOnly"`
	SameSite         string  `json:"sameSite"`
	Session          bool    `json:"session"`
	FirstPartyDomain string  `json:"firstPartyDomain"`
	PartitionKey     any     `json:"partitionKey"`
	StoreID          any     `json:"storeId"`
	ExpirationDate   float64 `json:"expirationDate,omitempty"`
}

// ParseExportedCookies reads cookies from a JSON export crated with Cookie-Editor.
func ParseExportedCookies(exported string) ([]http.Cookie, error) {
	var decoded ExportedCookies
	if err := json.NewDecoder(strings.NewReader(exported)).Decode(&decoded); err != nil {
		return nil, err
	}

	var cookies []http.Cookie
	for _, cookie := range decoded {
		if !strings.HasSuffix(cookie.Domain, "strava.com") {
			continue
		}

		log.Println("parsed cookie:", cookie.Name)

		var sameSite http.SameSite
		switch cookie.SameSite {
		case "lax":
			sameSite = http.SameSiteLaxMode
		case "strict":
			sameSite = http.SameSiteStrictMode
		default:
			sameSite = http.SameSiteDefaultMode
		}

		//nolint:exhaustruct
		cookies = append(cookies, http.Cookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Path:     cookie.Path,
			Domain:   cookie.Domain,
			Secure:   cookie.Secure,
			HttpOnly: cookie.HTTPOnly,
			SameSite: sameSite,
		})
	}

	return cookies, nil
}
