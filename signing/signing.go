// Package signing provides signing functionality for requests to/from Slack.
package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	slackTimestamp = "X-Slack-Request-Timestamp"
	slackSignature = "X-Slack-Signature"
)

// Request represents the pieces of a request needed to do a signature
// validation. We take this instead of an *http.Request so that we don't need to
// be responsible for rewinding its response body.
type Request struct {
	// T is the X-Slack-Request-Timestamp HTTP request header
	T string

	// S is the X-Slack-Signature HTTP request header, which is the provided
	// HMAC signature
	S string

	// B is the request body
	B []byte
}

func parseTimestamp(r Request) (int64, error) {
	ts, err := strconv.ParseInt(r.T, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse %s header: %w", slackTimestamp, err)
	}

	if time.Now().Unix()-ts > 300 { // was this at least 5
		return -1, fmt.Errorf("request timestamp (%d) too old", ts)
	}

	return ts, nil
}

func genHMAC(key, data string) string {
	m := hmac.New(sha256.New, []byte(key))

	if _, err := m.Write([]byte(data)); err != nil {
		panic(err.Error())
	}

	return fmt.Sprintf("v0=%x", m.Sum(nil))
}

// Validate takes the signature key, and the pieces of a request that allow us
// to validate its signature. If this returned an error, the validation failed.
// Returned errors are meant to be logged, not to be sent back to the entity
// making the request.
func Validate(key string, r Request) error {
	if len(r.T) == 0 {
		return fmt.Errorf("%s header not present", slackTimestamp)
	}

	if len(r.S) == 0 {
		return fmt.Errorf("%s header not present", slackSignature)
	}

	ts, err := parseTimestamp(r)
	if err != nil {
		return err
	}

	mac := genHMAC(key, fmt.Sprintf("v0:%d:%s", ts, string(r.B)))

	if hmac.Equal([]byte(r.S), []byte(mac)) {
		return nil
	}

	return errors.New("signature does not match")
}
