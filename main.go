package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
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
	"path"
	"strconv"

	_ "golang.org/x/image/webp"

	"github.com/anthonynsimon/bild/adjust"
	"github.com/anthonynsimon/bild/transform"
	"github.com/emersion/go-appdir"
	"github.com/go-chi/chi/v5"
)

type Flags struct {
	ListenURL string
	UserAgent string
	NoCache   bool
	Cookies   string
}

func main() {
	flags := ParseFlags()

	var cookies []http.Cookie

	if err := os.MkdirAll(CacheDir(), 0755); err != nil {
		log.Println("error creating cache directory:", err.Error())
		return
	}

	cookieFilename := path.Join(CacheDir(), "cookies")
	readCookies := false
	dumpCookies := false

	cookieFile, err := os.Open(cookieFilename)
	if os.IsNotExist(err) && !flags.NoCache {
		dumpCookies = true
	} else if err != nil {
		log.Println("error opening cookie cache:", err.Error())
		return
	} else {
		readCookies = true
	}
	defer func(c io.Closer) {
		if err := c.Close(); err != nil {
			log.Println("error closing cookie file:", err.Error())
		}
	}(cookieFile)

	if readCookies {
		log.Println("reading cookies from cache:", cookieFilename)

		if err := json.NewDecoder(cookieFile).Decode(&cookies); err != nil {
			log.Println("error reading cached cookies:", err.Error())
			return
		}
	} else {
		log.Println("loading cookies from Firefox")

		if flags.Cookies == "" {
			browserCookies, err := GetBrowserCookies()
			if err != nil {
				log.Println("error loading browser cookies:", err.Error())
				return
			}
			sessionCookies, err := GetSessionCookies()
			if err != nil {
				log.Println("error loading session cookies:", err.Error())
				return
			}
			cookies = append(cookies, browserCookies...)
			cookies = append(cookies, sessionCookies...)
		} else {
			log.Println("parsing cookies from CLI flags")

			exportedCookies, err := ParseExportedCookies(flags.Cookies)
			if err != nil {
				log.Println("error parsing exported cookies:", err.Error())
				return
			}
			cookies = append(cookies, exportedCookies...)
		}

	}

	if dumpCookies {
		log.Println("writing cookies to cache:", cookieFilename)

		cookieFile, err := os.Create(cookieFilename)
		if err != nil {
			log.Println("error creating cookie cache:", err.Error())
			return
		}
		if err := json.NewEncoder(cookieFile).Encode(cookies); err != nil {
			log.Println("error dumping cached cookies:", err.Error())
			return
		}
	}

	r := chi.NewRouter()

	r.Get("/{sport}/{z}/{x}/{y}.png", func(w http.ResponseWriter, r *http.Request) {
		sport := chi.URLParam(r, "sport")
		z, err1 := strconv.Atoi(chi.URLParam(r, "z"))
		x, err2 := strconv.Atoi(chi.URLParam(r, "x"))
		y, err3 := strconv.Atoi(chi.URLParam(r, "y"))
		if errors.Join(err1, err2, err3) != nil {
			log.Println("invalid URL parameters")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		hue, _ := strconv.Atoi(r.URL.Query().Get("hue"))
		saturation, _ := strconv.ParseFloat(r.URL.Query().Get("saturation"), 64)

		tile, err := GetTile(sport, z, x, y, flags, cookies)
		if err != nil {
			log.Println("error fetching tile from Strava:", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tile = TilePipeline(tile, hue, saturation)

		if err := png.Encode(w, tile); err != nil {
			log.Println("error encoding tile:", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	log.Println("starting HTTP server on", flags.ListenURL)
	if err := http.ListenAndServe(flags.ListenURL, r); err != nil {
		log.Println("error starting local server", err)
		return
	}

}

func GetTile(
	sport string,
	z int,
	x int,
	y int,
	flags Flags,
	cookies []http.Cookie,
) (image.Image, error) {
	if !flags.NoCache {
		filename := CacheFileName(z, x, y, sport, false)
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

		tile, err := GetTile(sport, 15, tileX, tileY, flags, cookies)
		if err != nil {
			return nil, err
		}

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
		sport,
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
		if err := os.MkdirAll(CacheFileName(z, x, y, sport, true), 0755); err != nil {
			return nil, err
		}

		if err := os.WriteFile(CacheFileName(z, x, y, sport, false), buf, 0644); err != nil {
			return nil, err
		}
	}

	return tile, nil
}

func TilePipeline(tile image.Image, hue int, saturation float64) image.Image {
	if hue != 0 {
		tile = adjust.Hue(tile, hue)
	}
	if saturation != 0 {
		tile = adjust.Saturation(tile, saturation)
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
	noCache := flag.Bool(
		"nocache",
		false,
		"disable caching of downloaded tiles and cookies",
	)
	cookies := flag.String(
		"cookies",
		"",
		"JSON cookies exported using Cookie-Editor",
	)

	flag.Parse()

	return Flags{
		ListenURL: *listenURL,
		UserAgent: *userAgent,
		NoCache:   *noCache,
		Cookies:   *cookies,
	}
}
