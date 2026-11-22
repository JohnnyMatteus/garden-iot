package openmeteo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/JohnnyMatteus/garden-iot/services/irrigation/domain/weather"
)

const defaultBaseURL = "https://api.open-meteo.com/v1/forecast"

// Client implementa application.WeatherProvider.
type Client struct {
	BaseURL    string
	HTTP       *http.Client
	Translator WeatherTranslator
}

func NewClient() *Client {
	return &Client{BaseURL: defaultBaseURL, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) Forecast(ctx context.Context, lat, lon float64, from time.Time, horizon time.Duration) (weather.Forecast, error) {
	q := url.Values{
		"latitude": {strconv.FormatFloat(lat, 'f', 4, 64)}, "longitude": {strconv.FormatFloat(lon, 'f', 4, 64)},
		"hourly":          {"temperature_2m,precipitation,wind_speed_10m"},
		"wind_speed_unit": {"ms"}, "timeformat": {"unixtime"}, "timezone": {"UTC"},
		"forecast_days": {strconv.Itoa(int(horizon.Hours()/24) + 2)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return weather.Forecast{}, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return weather.Forecast{}, fmt.Errorf("open-meteo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return weather.Forecast{}, fmt.Errorf("open-meteo: status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return weather.Forecast{}, err
	}
	return c.Translator.Translate(raw, from, horizon)
}
