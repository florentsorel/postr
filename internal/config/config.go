package config

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/caarlos0/env/v11"
	"github.com/florentsorel/postr/internal/mediaserver"
)

type Config struct {
	// MediaServer selects the active media server: "plex" or "jellyfin".
	// When empty it is inferred from whichever server's URL is set, defaulting
	// to Plex so existing deployments keep working untouched.
	MediaServer string `env:"MEDIA_SERVER"`

	PlexURL   string `env:"PLEX_URL"`
	PlexToken string `env:"PLEX_TOKEN"`

	JellyfinURL    string `env:"JELLYFIN_URL"`
	JellyfinAPIKey string `env:"JELLYFIN_API_KEY"`

	AuthEnabled bool   `env:"AUTH_ENABLED"`
	AuthUser    string `env:"AUTH_USER"`
	AuthPass    string `env:"AUTH_PASS"`

	TmdbAPIKey   string `env:"TMDB_API_KEY"`
	TvdbAPIKey   string `env:"TVDB_API_KEY"`
	FanartAPIKey string `env:"FANART_API_KEY"`

	DBPath   string `env:"DB_PATH"   envDefault:"data/postr.db"`
	DataPath string `env:"DATA_PATH" envDefault:"data"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	if err := cfg.normalize(); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ServerURL returns the base URL of the active media server.
func (c *Config) ServerURL() string {
	if c.MediaServer == mediaserver.ProviderJellyfin {
		return c.JellyfinURL
	}
	return c.PlexURL
}

// ServerToken returns the credential of the active media server: a Plex token
// or a Jellyfin API key.
func (c *Config) ServerToken() string {
	if c.MediaServer == mediaserver.ProviderJellyfin {
		return c.JellyfinAPIKey
	}
	return c.PlexToken
}

// ServerConfigured reports whether the active media server has both a URL and a
// credential, which is the minimum needed to attempt a connection.
func (c *Config) ServerConfigured() bool {
	return c.ServerURL() != "" && c.ServerToken() != ""
}

// ServerName returns the display name of the active media server.
func (c *Config) ServerName() string {
	if c.MediaServer == mediaserver.ProviderJellyfin {
		return "Jellyfin"
	}
	return "Plex"
}

func (c *Config) normalize() error {
	if c.MediaServer == "" {
		// No explicit choice: pick the server that actually has a URL, keeping
		// Plex as the default for existing deployments.
		if c.JellyfinURL != "" && c.PlexURL == "" {
			c.MediaServer = mediaserver.ProviderJellyfin
		} else {
			c.MediaServer = mediaserver.ProviderPlex
		}
	}
	if c.MediaServer != mediaserver.ProviderPlex && c.MediaServer != mediaserver.ProviderJellyfin {
		return fmt.Errorf("invalid MEDIA_SERVER %q: must be %q or %q",
			c.MediaServer, mediaserver.ProviderPlex, mediaserver.ProviderJellyfin)
	}

	var err error
	if c.PlexURL, err = normalizeURL(c.PlexURL, "PLEX_URL"); err != nil {
		return err
	}
	if c.JellyfinURL, err = normalizeURL(c.JellyfinURL, "JELLYFIN_URL"); err != nil {
		return err
	}
	return nil
}

// normalizeURL defaults the scheme to http:// when omitted and strips any path,
// query or fragment so only the origin remains.
func normalizeURL(raw, name string) (string, error) {
	if raw == "" {
		return "", nil
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		// No scheme — try prepending http://
		u, err = url.Parse("http://" + raw)
		if err != nil {
			return "", fmt.Errorf("invalid %s %q: %w", name, raw, err)
		}
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid %s %q: scheme must be http or https", name, raw)
	}

	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func (c *Config) validate() error {
	if c.AuthEnabled {
		if c.AuthUser == "" {
			return errors.New("AUTH_USER must be set when AUTH_ENABLED is true")
		}
		if c.AuthPass == "" {
			return errors.New("AUTH_PASS must be set when AUTH_ENABLED is true")
		}
	}
	return nil
}
