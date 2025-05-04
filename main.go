package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/firefox"
	"github.com/go-chi/chi/v5"
)

func main() {
	flags := parseFlags()

	cookieJar, err := kooky.ReadCookies(
		context.Background(),
		kooky.Valid,
		kooky.DomainHasSuffix("strava.com"),
	)
	if err != nil {
		log.Println("cannot read cookies from browser:", err.Error())
		os.Exit(1)
	}

	var cookies []http.Cookie

	for _, cookie := range cookieJar {
		cookies = append(cookies, cookie.Cookie)

		log.Println("found cookie:", cookie.Name)
	}

	sessionCookieJars, err := GetSessionCookieJars()
	if err != nil {
		log.Println("error extracting firefox session cookies:", err)
		return
	}

	for _, sessionCookieJar := range sessionCookieJars {
		for _, cookie := range sessionCookieJar.Cookies {
			if !strings.HasSuffix(cookie.Host, "strava.com") {
				continue
			}

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

	r := chi.NewRouter()

	r.Get("/{z}/{x}/{y}.png", func(w http.ResponseWriter, r *http.Request) {
		z := chi.URLParam(r, "z")
		x := chi.URLParam(r, "x")
		y := chi.URLParam(r, "y")

		tile, err := GetTile(z, x, y, flags.Sport, flags.UserAgent, cookies)
		if err != nil {
			log.Println("error fetching tile from Strava:", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		png.Encode(w, tile)
	})

	if flags.CertFile == "" || flags.KeyFile == "" {
		log.Println("starting HTTP server on", flags.ListenURL)

		if err := http.ListenAndServe(flags.ListenURL, r); err != nil {
			log.Println("error starting local server", err)
			return
		}
	} else {
		log.Println("starting HTTPS server on", flags.ListenURL)
		log.Println("using certificate file", flags.CertFile)
		log.Println("using certificate key file", flags.KeyFile)

		if err := http.ListenAndServeTLS(flags.ListenURL, flags.CertFile, flags.KeyFile, r); err != nil {
			log.Println("error starting local server", err)
			return
		}
	}

}

func GetTile(z, x, y, sport, userAgent string, cookies []http.Cookie) (image.Image, error) {
	url := fmt.Sprintf("https://content-a.strava.com/identified/globalheat/%s/mobileblue/%s/%s/%s.png?v=2", sport, z, x, y)
	log.Println("fetching", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	for _, c := range cookies {
		req.AddCookie(&c)
	}

	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Accept", "image/webp,*/*")
	req.Header.Add("Referer", "https://www.strava.com/")
	req.Header.Add("Origin", "https://www.strava.com")

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, errors.New("received non-200 status code from strava tile server")
	}

	tile, _, err := image.Decode(response.Body)
	if err != nil {
		return nil, err
	}

	if err := response.Body.Close(); err != nil {
		log.Println("error closing response body:", err.Error())
	}

	return tile, nil
}

type Flags struct {
	Sport     string
	ListenURL string
	UserAgent string
	CertFile  string
	KeyFile   string
}

func parseFlags() Flags {
	sport := flag.String(
		"sport",
		"sport_Ride",
		"internal sport identifier used by Strava",
	)
	listenURL := flag.String(
		"listen",
		"localhost:8080",
		"listen URL used for the HTTP(s) server",
	)
	userAgent := flag.String(
		"useragent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:138.0) Gecko/20100101 Firefox/138.0",
		"user agent to use for HTTP requests",
	)
	certFile := flag.String(
		"certfile",
		"",
		"certificate file for the built-in HTTPS server",
	)
	keyFile := flag.String(
		"keyfile",
		"",
		"certificate key file for the built-in HTTPS server",
	)

	flag.Parse()

	return Flags{
		Sport:     *sport,
		ListenURL: *listenURL,
		UserAgent: *userAgent,
		CertFile:  *certFile,
		KeyFile:   *keyFile,
	}
}
