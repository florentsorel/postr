package config

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	PlexURL   string `env:"PLEX_URL"`
	PlexToken string `env:"PLEX_TOKEN"`

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

func (c *Config) normalize() error {
	if c.PlexURL == "" {
		return nil
	}

	u, err := url.Parse(c.PlexURL)
	if err != nil || u.Host == "" {
		// No scheme — try prepending http://
		u, err = url.Parse("http://" + c.PlexURL)
		if err != nil {
			return fmt.Errorf("invalid PLEX_URL %q: %w", c.PlexURL, err)
		}
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid PLEX_URL %q: scheme must be http or https", c.PlexURL)
	}

	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	c.PlexURL = u.String()
	return nil
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
