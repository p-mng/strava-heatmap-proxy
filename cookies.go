package main

import (
	"context"
	"log"
	"net/http"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/firefox"
)

// GetBrowserCookies extracts all cookies from Firefox.
func GetBrowserCookies() ([]http.Cookie, error) {
	cookieJar, err := kooky.ReadCookies(
		context.Background(),
		kooky.Valid,
		kooky.DomainHasSuffix("strava.com"),
	)
	if err != nil {
		return nil, err
	}

	var cookies []http.Cookie

	for _, cookie := range cookieJar {
		cookies = append(cookies, cookie.Cookie)
		log.Println("found cookie:", cookie.Name)
	}

	return cookies, nil
}
