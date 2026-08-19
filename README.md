# Strava Heatmap Proxy

A utility program that serves a local proxy for the Strava heatmap, making it accessible in other programs like BRouter (e.g., `bikerouter.de` and `brouter.de`), GIS software (e.g., QGIS), and OpenStreetMap editors (e.g., iD).

![Screenshot](./assets/screenshot.png)

> [!IMPORTANT]
> Currently, only macOS/Linux and Firefox are supported. While there might be workarounds for getting the proxy to run on other setups, help with porting the program to Windows and Chromium-based browsers is always welcome!

## Installation

The latest version can be installed using `go install github.com/p-mng/strava-heatmap-proxy@latest`.

## Usage

- Start Firefox and log into your Strava account.
- Start the proxy (e.g., using `strava-heatmap-proxy -sport sport_MountainBikeRide`). See the help below for an explanation of all available options.
  - By default, the program will try to extract the required cookies from Firefox (only works on macOS and Linux).
  - If this fails (because you use Windows, Chrome, or a private session), you can manually export cookies using Cookie-Editor extension (download for [Firefox](https://addons.mozilla.org/en-US/firefox/addon/cookie-editor/) or [Chrome](https://chromewebstore.google.com/detail/cookie-editor/hlkenndednhfkekhgcdicdfddnkalmdm)) as JSON and pass them to the program.
  - Cookies are cached by default, so on subsequent launches you don't need to extract or export cookies again.
- Add the tile layer/overlay to your program of choice using the following URL: `http://localhost:8080/{z}/{x}/{y}.png`.

```
Usage of strava-heatmap-proxy:
  -certfile string
        certificate file for the built-in HTTPS server
  -cookies string
        JSON cookies exported using Cookie-Editor
  -hue int
        shift the hue of the heatmap from the default blue, measured in degrees
  -keyfile string
        certificate key file for the built-in HTTPS server
  -listen string
        listen URL used for the HTTP(s) server (default "localhost:8080")
  -nocache
        disable caching of downloaded tiles and cookies
  -saturation float
        adjusts the saturation of the image, with -1.0 being -100% and 1.0 being 100%
  -sport string
        internal sport identifier used by Strava (default "sport_Ride")
  -useragent string
        user agent to use for HTTP requests (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:138.0) Gecko/20100101 Firefox/138.0")
```

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.
