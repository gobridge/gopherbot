// Package config provides the configuration helpers for gopher, for pulling
// configuration from the environment.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
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

// S is the Slack environment configuration
type S struct {
	// AppID is the Slack App ID
	// Env: SLACK_APP_ID
	AppID string

	// ClientID is the Client ID
	// Env: SLACK_CLIENT_ID
	ClientID string

	// ClientSecret is the Client secret
	// Env: SLACK_CLIENT_SECRET
	ClientSecret string

	// RequestSecret is the HMAC signing secret used for Slack request signing
	// Env: SLACK_REQUEST_SECRET
	RequestSecret string

	// RequestToken is the Slack verification token
	// Env: SLACK_REQUEST_TOKEN
	RequestToken string
}

// C is the configuration struct.
type C struct {
	// LogLevel is the logging level
	// Env: LOG_LEVEL
	LogLevel zerolog.Level

	// Env is the current environment.
	// Env: ENV
	Env Environment

	// Port is the TCP port for web workers to listen on, loaded from PORT
	// Env: PORT
	Port uint16

	// Heroku are the Labs Dyno Metadata environment variables
	Heroku H

	// Redis is the Redis configuration, loaded from REDIS_URL
	Redis R

	// Slack is the Slack configuration, loaded from a few SLACK_* environment
	// variables
	Slack S
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

	ll := os.Getenv("LOG_LEVEL")
	if len(ll) == 0 {
		ll = "info"
	}

	l, err := zerolog.ParseLevel(ll)
	if err != nil {
		return C{}, fmt.Errorf("failed to parse level: %w", err)
	}

	c.LogLevel = l
	c.Env = strToEnv(os.Getenv("ENV"))

	c.Heroku.AppID = os.Getenv("HEROKU_APP_ID")
	c.Heroku.AppName = os.Getenv("HEROKU_APP_NAME")
	c.Heroku.DynoID = os.Getenv("HEROKU_DYNO_ID")

	c.Slack.AppID = os.Getenv("SLACK_APP_ID")
	c.Slack.ClientID = os.Getenv("SLACK_CLIENT_ID")
	c.Slack.RequestToken = os.Getenv("SLACK_REQUEST_TOKEN")

	c.Slack.ClientSecret = os.Getenv("SLACK_CLIENT_SECRET")
	c.Slack.RequestSecret = os.Getenv("SLACK_REQUEST_SECRET")

	_ = os.Unsetenv("SLACK_CLIENT_SECRET")  // paranoia
	_ = os.Unsetenv("SLACK_REQUEST_SECRET") // paranois

	return c, nil
}
