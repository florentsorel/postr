package config

import (
	"errors"

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
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
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
