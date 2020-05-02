// Package signing provides signing functionality for requests to/from Slack.
package signing

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

const (
	// SlackTimestampHeader is the HTTP header that Slack uses for specifying
	// the timestamp used in generating the signature.
	SlackTimestampHeader = "X-Slack-Request-Timestamp"

	// SlackSignatureHeader is the HTTP header that Slack uses for specifying
	// the signature that was generated.
	SlackSignatureHeader = "X-Slack-Signature"
)

// Request represents the pieces of a request needed to do a signature
// validation. We take this instead of an *http.Request so that we don't need to
// be responsible for rewinding its response body.
type Request struct {
	// Timestamp is the X-Slack-Request-Timestamp HTTP request header
	Timestamp string

	// Signature is the X-Slack-Signature HTTP request header, which is the provided
	// HMAC signature
	Signature string

	// Body is the request body
	Body []byte
}

func parseTimestamp(t string) (int64, error) {
	ts, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse %s header: %w", SlackTimestampHeader, err)
	}

	if time.Now().Unix()-ts > 300 { // was this more than 5 minutes ago?
		return -1, fmt.Errorf("request timestamp (%d) too old", ts)
	}

	return ts, nil
}

func genHMAC(key, data string) string {
	m := hmac.New(sha256.New, []byte(key))

	// if this fails, we have bigger problems
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
	if len(r.Timestamp) == 0 {
		return fmt.Errorf("%s header not present", SlackTimestampHeader)
	}

	if len(r.Signature) == 0 {
		return fmt.Errorf("%s header not present", SlackSignatureHeader)
	}

	ts, err := parseTimestamp(r.Timestamp)
	if err != nil {
		return err
	}

	mac := genHMAC(key, fmt.Sprintf("v0:%d:%s", ts, string(r.Body)))

	if hmac.Equal([]byte(r.Signature), []byte(mac)) {
		return nil
	}

	return errors.New("signature does not match")
}

// Sign takes the signature key, and a request, and then signs the request using
// that key. Afterwards the request should be trusted by any application
// implementing this signature verification method.
func Sign(key string, r *http.Request) error {
	var b []byte

	if r.Body != nil {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}

		b = body

		// reset the body
		r.Body = ioutil.NopCloser(bytes.NewReader(b))
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)

	r.Header.Set("X-Slack-Request-Timestamp", ts)
	r.Header.Set("X-Slack-Signature", genHMAC(key, fmt.Sprintf("v0:%s:%s", ts, b)))

	return nil
}
