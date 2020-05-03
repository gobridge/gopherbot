// Package config provides the configuration helpers for gopher, for pulling
// configuration from the environment.
package config

import (
	"fmt"
	"net"
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

	// Insecure is whether we should connect to Redis over plain text
	Insecure bool

	// SkipVerify is whether we skip x.509 certification validation
	SkipVerify bool
}

// H is the Heroku environment configuration
type H struct {
	// AppID is the HEROKU_APP_ID
	AppID string

	// AppName is the HEROKU_APP_NAME
	AppName string

	// DynoID is the HEROKU_DYNO_ID
	DynoID string

	// Commit is the HEROKU_SLUG_COMMIT
	Commit string
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

func secureRedisCredentials(s string, insecure bool) (host, user, password string, err error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", "", "", err
	}

	switch u.Scheme {
	case "rediss":
		pass, _ := u.User.Password()
		return u.Host, u.User.Username(), pass, nil

	case "redis":
		h, p, err := net.SplitHostPort(u.Host)
		if err != nil {
			if !strings.Contains(err.Error(), "missing port in address") {
				return "", "", "", err
			}

			h = u.Host
		}

		if p == "" {
			p = "6379"
		}

		pi, err := strconv.Atoi(p)
		if err != nil {
			return "", "", "", err
		}

		if !insecure { // it's secure
			pi++
		}

		pass, _ := u.User.Password()

		return net.JoinHostPort(h, strconv.Itoa(pi)), u.User.Username(), pass, nil

	default:
		return "", "", "", fmt.Errorf("unknown scheme: %s", u.Scheme)
	}
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
		c.Redis.Insecure = os.Getenv("REDIS_INSECURE") == "1"
		c.Redis.SkipVerify = os.Getenv("REDIS_SKIPVERIFY") == "1"

		a, u, p, err := secureRedisCredentials(r, c.Redis.Insecure)
		if err != nil {
			return C{}, fmt.Errorf("failed to parse REDIS_URL: %w", err)
		}

		c.Redis.Addr = a
		c.Redis.User = u
		c.Redis.Password = p
	}

	ll := os.Getenv("LOG_LEVEL")
	if len(ll) == 0 {
		ll = "info"
	}

	l, err := zerolog.ParseLevel(ll)
	if err != nil {
		return C{}, fmt.Errorf("failed to parse LOG_LEVEL: %w", err)
	}

	c.LogLevel = l
	c.Env = strToEnv(os.Getenv("ENV"))

	c.Heroku.AppID = os.Getenv("HEROKU_APP_ID")
	c.Heroku.AppName = os.Getenv("HEROKU_APP_NAME")
	c.Heroku.DynoID = os.Getenv("HEROKU_DYNO_ID")
	c.Heroku.Commit = os.Getenv("HEROKU_SLUG_COMMIT")

	c.Slack.AppID = os.Getenv("SLACK_APP_ID")
	c.Slack.ClientID = os.Getenv("SLACK_CLIENT_ID")
	c.Slack.RequestToken = os.Getenv("SLACK_REQUEST_TOKEN")

	c.Slack.ClientSecret = os.Getenv("SLACK_CLIENT_SECRET")
	c.Slack.RequestSecret = os.Getenv("SLACK_REQUEST_SECRET")

	_ = os.Unsetenv("SLACK_CLIENT_SECRET")  // paranoia
	_ = os.Unsetenv("SLACK_REQUEST_SECRET") // paranois

	return c, nil
}
