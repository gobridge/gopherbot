// Package config provides the configuration helpers for gopher, for pulling
// configuration from the environment.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Environment is the current runtime environment.
type Environment string

const (
	// Development is for when it's the development environment
	Development Environment = "development"

	// Testing is WISOTT
	Testing Environment = "testing"

	// Staging is WISOTT
	Staging Environment = "staging"

	// Production is WISOTT
	Production Environment = "production"
)

func strToEnv(s string) Environment {
	switch strings.ToLower(s) {
	case "production":
		return Production
	case "staging":
		return Staging
	case "testing":
		return Testing
	default:
		return Development
	}
}

// R are the Redis-specific options.
type R struct {
	// Addr is the Redis host and port to connect to
	Addr string

	// User is the Redis user
	User string

	// Password is the Redis password
	Password string
}

// H is the Heroku environment configuration
type H struct {
	// AppID is the Heroku application ID
	AppID string

	// AppName is the Heroku application name
	AppName string

	// DynoID is the Heroku Dyno ID
	DynoID string
}

// C is the configuration struct.
type C struct {
	// Env is the current environment.
	Env Environment

	// Port is the TCP port for web workers to listen on, loaded from PORT
	Port uint16

	// Heroku are the Labs Dyno Metadata environment variables
	Heroku H

	// Redis is the Redis configuration, loaded from REDIS_URL
	Redis R
}

// LoadEnv loads the configuration from the appropriate environment variables.
func LoadEnv() (C, error) {
	var c C

	if p := os.Getenv("PORT"); len(p) > 0 {
		u, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return C{}, fmt.Errorf("failed to parse PORT: %w", err)
		}

		c.Port = uint16(u)
	}

	if r := os.Getenv("REDIS_URL"); len(r) > 0 {
		rurl, err := url.Parse(r)
		if err != nil {
			return C{}, fmt.Errorf("failed to parse REDIS_URL: %w", err)
		}

		c.Redis.Addr = rurl.Host
		c.Redis.User = rurl.User.Username()

		if p, ok := rurl.User.Password(); ok {
			c.Redis.Password = p
		}
	}

	c.Env = strToEnv(os.Getenv("ENV"))
	c.Heroku.AppID = os.Getenv("HEROKU_APP_ID")
	c.Heroku.AppName = os.Getenv("HEROKU_APP_NAME")
	c.Heroku.DynoID = os.Getenv("HEROKU_DYNO_ID")

	return c, nil
}
