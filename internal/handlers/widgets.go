package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Bangalore coordinates, hardcoded for the weather widget.
const (
	bangaloreLat = 12.9716
	bangaloreLon = 77.5946
)

// istLocation is Asia/Kolkata; falls back to a fixed +5:30 zone if tzdata is
// unavailable (e.g. a minimal container).
var istLocation = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Kolkata"); err == nil {
		return loc
	}
	return time.FixedZone("IST", 5*3600+30*60)
}()

// Clock returns the current time/date/day in IST.
func (h *Handlers) Clock(w http.ResponseWriter, r *http.Request) {
	now := time.Now().In(istLocation)
	writeJSON(w, http.StatusOK, map[string]string{
		"time": now.Format("15:04"),
		"date": strings.ToUpper(now.Format("Mon 02 Jan")),
		"day":  now.Format("Monday"),
	})
}

// owmResponse is the subset of the OpenWeatherMap current-weather response we use.
type owmResponse struct {
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Weather []struct {
		Main string `json:"main"`
	} `json:"weather"`
}

// weatherIcon maps an OWM "main" condition group to a simple icon keyword.
func weatherIcon(main string) string {
	switch main {
	case "Clear":
		return "sun"
	case "Clouds":
		return "cloud"
	case "Rain", "Drizzle":
		return "rain"
	case "Thunderstorm":
		return "storm"
	case "Snow":
		return "snow"
	case "Mist", "Fog", "Haze", "Smoke", "Dust", "Sand":
		return "fog"
	default:
		return "cloud"
	}
}

// Weather fetches current Bangalore weather from OpenWeatherMap (free tier,
// current-weather endpoint) and returns a compact display payload.
func (h *Handlers) Weather(w http.ResponseWriter, r *http.Request) {
	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		writeError(w, http.StatusServiceUnavailable, "OPENWEATHER_API_KEY not configured")
		return
	}

	url := fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/weather?lat=%f&lon=%f&units=metric&appid=%s",
		bangaloreLat, bangaloreLon, apiKey,
	)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		writeError(w, http.StatusBadGateway, "weather upstream unreachable")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway,
			fmt.Sprintf("weather upstream returned %d", resp.StatusCode))
		return
	}

	var data owmResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadGateway, "could not parse weather response")
		return
	}

	condition := "Unknown"
	icon := "cloud"
	if len(data.Weather) > 0 {
		condition = data.Weather[0].Main
		icon = weatherIcon(data.Weather[0].Main)
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"temp":       fmt.Sprintf("%.0f°C", data.Main.Temp),
		"condition":  condition,
		"humidity":   fmt.Sprintf("%d%%", data.Main.Humidity),
		"feels_like": fmt.Sprintf("%.0f°", data.Main.FeelsLike),
		"icon":       icon,
	})
}

// lastfmResponse is the subset of user.getrecenttracks we care about.
type lastfmResponse struct {
	RecentTracks struct {
		Track []lastfmTrack `json:"track"`
	} `json:"recenttracks"`
}

type lastfmTrack struct {
	Name   string `json:"name"`
	Artist struct {
		Text string `json:"#text"`
	} `json:"artist"`
	Album struct {
		Text string `json:"#text"`
	} `json:"album"`
	Attr struct {
		NowPlaying string `json:"nowplaying"`
	} `json:"@attr"`
}

// Spotify serves Now Playing data from Last.fm (which scrobbles Apple Music).
// The route name is historical; the source is Last.fm user.getrecenttracks.
//
// Responses:
//   {"status":"not_configured"}          — env vars missing
//   {"status":"idle"}                    — nothing playing / upstream issue
//   {"status":"playing","track":..,"artist":..,"album":..}
func (h *Handlers) Spotify(w http.ResponseWriter, r *http.Request) {
	apiKey := os.Getenv("LASTFM_API_KEY")
	username := os.Getenv("LASTFM_USERNAME")
	if apiKey == "" || username == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "not_configured"})
		return
	}

	q := url.Values{}
	q.Set("method", "user.getrecenttracks")
	q.Set("user", username)
	q.Set("api_key", apiKey)
	q.Set("format", "json")
	q.Set("limit", "1")
	endpoint := "http://ws.audioscrobbler.com/2.0/?" + q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "idle"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusOK, map[string]string{"status": "idle"})
		return
	}

	var data lastfmResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "idle"})
		return
	}

	if len(data.RecentTracks.Track) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "idle"})
		return
	}

	t := data.RecentTracks.Track[0]
	if t.Attr.NowPlaying != "true" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "idle"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "playing",
		"track":  t.Name,
		"artist": t.Artist.Text,
		"album":  t.Album.Text,
	})
}
