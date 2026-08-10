package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/anthonynsimon/bild/adjust"
	"github.com/anthonynsimon/bild/transform"
	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/firefox"
	"github.com/emersion/go-appdir"
	"github.com/go-chi/chi/v5"
)

type Flags struct {
	Sport      string
	ListenURL  string
	UserAgent  string
	CertFile   string
	KeyFile    string
	Hue        int
	Saturation float64
	NoCache    bool
}

func main() {
	flags := ParseFlags()

	log.Println("read flags:", flags)

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

	r := chi.NewRouter()

	r.Get("/{z}/{x}/{y}.png", func(w http.ResponseWriter, r *http.Request) {
		z, err1 := strconv.Atoi(chi.URLParam(r, "z"))
		x, err2 := strconv.Atoi(chi.URLParam(r, "x"))
		y, err3 := strconv.Atoi(chi.URLParam(r, "y"))
		if errors.Join(err1, err2, err3) != nil {
			log.Println("invalid URL parameters")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tile, err := GetTile(z, x, y, flags, cookies)
		if err != nil {
			log.Println("error fetching tile from Strava:", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tile = TilePipeline(tile, flags)

		if err := png.Encode(w, tile); err != nil {
			log.Println("error encoding tile:", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
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

func GetTile(
	z int,
	x int,
	y int,
	flags Flags,
	cookies []http.Cookie,
) (image.Image, error) {
	if !flags.NoCache {
		filename := CacheFileName(z, x, y, flags.Sport, false)
		file, err := os.Open(filename)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		} else if err == nil {
			log.Println("loading cached tile from disk:", filename)
			tile, _, err := image.Decode(file)
			return tile, err
		}
	}

	// Strava servers do not offer zoom levels > 15
	if z > 15 {
		divisor := 1 << (z - 15)

		tileX := x / divisor
		tileY := y / divisor

		tile, _ := GetTile(15, tileX, tileY, flags, cookies)

		newWidth := 512 / divisor
		offsetX := x % divisor * newWidth
		offsetY := y % divisor * newWidth

		rect := image.Rect(offsetX, offsetY, offsetX+newWidth, offsetY+newWidth)
		log.Println("cropping tile:", rect)

		cropped := transform.Crop(tile, rect)
		return cropped, nil
	}

	url := fmt.Sprintf(
		"https://content-a.strava.com/identified/globalheat/%s/mobileblue/%d/%d/%d.png?v=2",
		flags.Sport,
		z,
		x,
		y,
	)
	log.Println("cached tile does not exist, downloading from Strava:", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	for _, c := range cookies {
		req.AddCookie(&c)
	}

	req.Header.Add("User-Agent", flags.UserAgent)
	req.Header.Add("Accept", "image/webp,*/*")
	req.Header.Add("Referer", "https://www.strava.com/")
	req.Header.Add("Origin", "https://www.strava.com")

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Println("error closing response body:", err.Error())
		}
	}()

	if response.StatusCode != http.StatusOK {
		return nil, errors.New("received non-200 status code from Strava tile server")
	}

	buf, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	tile, _, err := image.Decode(bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}

	// only save to cache after we know the image is good
	if !flags.NoCache {
		if err := os.MkdirAll(CacheFileName(z, x, y, flags.Sport, true), 0755); err != nil {
			return nil, err
		}

		if err := os.WriteFile(CacheFileName(z, x, y, flags.Sport, false), buf, 0644); err != nil {
			return nil, err
		}
	}

	return tile, nil
}

func TilePipeline(tile image.Image, flags Flags) image.Image {
	if flags.Hue != 0 {
		tile = adjust.Hue(tile, flags.Hue)
	}
	if flags.Saturation != 0 {
		tile = adjust.Saturation(tile, flags.Saturation)
	}
	return tile
}

func CacheDir() string {
	return appdir.New("strava_heatmap").UserCache()
}

func CacheFileName(z, x, y int, sport string, dir bool) string {
	if dir {
		return fmt.Sprintf("%s/%s/%d/%d", CacheDir(), sport, z, x)
	}
	return fmt.Sprintf("%s/%s/%d/%d/%d.png", CacheDir(), sport, z, x, y)
}

func ParseFlags() Flags {
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
	hue := flag.Int(
		"hue",
		0,
		"shift the hue of the heatmap from the default blue, measured in degrees",
	)
	saturation := flag.Float64(
		"saturation",
		0,
		"adjusts the saturation of the image, with -1.0 being -100% and 1.0 being 100%",
	)
	noCache := flag.Bool(
		"nocache",
		false,
		"disable caching of downloaded tiles",
	)

	flag.Parse()

	return Flags{
		Sport:      *sport,
		ListenURL:  *listenURL,
		UserAgent:  *userAgent,
		CertFile:   *certFile,
		KeyFile:    *keyFile,
		Hue:        *hue,
		Saturation: *saturation,
		NoCache:    *noCache,
	}
}
