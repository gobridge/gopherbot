package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// slackExampleSecret was taken from the Slack reference documentation on this process
const slackExampleSecret = "8f742231b10e8888abcd99yyyzzz85a5"

// testErrCheck looks to see if errContains is a substring of err.Error(). If
// not, this calls t.Fatal(). It also calls t.Fatal() if there was an error, but
// errContains is empty. Returns true if you should continue running the test,
// or false if you should stop the test.
func testErrCheck(t *testing.T, name string, errContains string, err error) bool {
	t.Helper()

	if len(errContains) > 0 {
		if err == nil {
			t.Fatalf("%s error = <nil>, should contain %q", name, errContains)
			return false
		}

		if errStr := err.Error(); !strings.Contains(errStr, errContains) {
			t.Fatalf("%s error = %q, should contain %q", name, errStr, errContains)
			return false
		}

		return false
	}

	if err != nil && len(errContains) == 0 {
		t.Fatalf("%s unexpected error: %v", name, err)
		return false
	}

	return true
}

func tgenHMAC(t *testing.T, s string) string {
	t.Helper()

	m := hmac.New(sha256.New, []byte(slackExampleSecret))

	_, err := m.Write([]byte(s))
	testErrCheck(t, "m.Write()", "", err)

	return fmt.Sprintf("v0=%x", m.Sum(nil))
}

func TestValidate(t *testing.T) {
	body := "testBody"
	now := strconv.Itoa(int(time.Now().Unix()))

	// generate signature without body
	sigA := tgenHMAC(t, fmt.Sprintf("v0:%s:%s", now, ""))

	// generate signature with the test body
	sigB := tgenHMAC(t, fmt.Sprintf("v0:%s:%s", now, body))

	tests := []struct {
		name string
		r    Request
		err  string
	}{
		{
			name: "missing_timestamp",
			err:  "X-Slack-Request-Timestamp header not present",
		},
		{
			name: "missing_signature",
			r: Request{
				Timestamp: "1531420618",
			},
			err: "X-Slack-Signature header not present",
		},
		{
			name: "garbage_timestamp",
			r: Request{
				Timestamp: "lol",
				Signature: "v0=55f41ec73231010289b54e669149ea021fccab11b5524355523533ce930cb739",
			},
			err: `failed to parse X-Slack-Request-Timestamp header: strconv.ParseInt: parsing "lol": invalid syntax`,
		},
		{
			name: "old_timestamp",
			r: Request{
				Timestamp: "1531420618",
				Signature: "v0=55f41ec73231010289b54e669149ea021fccab11b5524355523533ce930cb739",
			},
			err: "request timestamp (1531420618) too old",
		},
		{
			name: "ok_no_body",
			r: Request{
				Timestamp: now,
				Signature: sigA,
			},
		},
		{
			name: "ok_body",
			r: Request{
				Timestamp: now,
				Signature: sigB,
				Body:      []byte(body),
			},
		},
		{
			name: "wrong",
			r: Request{
				Timestamp: now,
				Signature: sigB + "x",
				Body:      []byte(body),
			},
			err: "signature does not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testErrCheck(t, "Validate()", tt.err, Validate(slackExampleSecret, tt.r))
		})
	}
}

type garbageRC struct{}

func (garbageRC) Read(_ []byte) (int, error) {
	return 0, errors.New("xxx")
}

func mustRequest(body io.Reader) *http.Request {
	r, err := http.NewRequest(http.MethodPost, "http://example.org/", body)
	if err != nil {
		panic(err)
	}

	return r
}

func TestSign(t *testing.T) {
	badBody := garbageRC{}
	goodBody := strings.NewReader("{}")

	tests := []struct {
		name string
		req  *http.Request
		err  string
	}{
		{
			name: "good",
			req:  mustRequest(goodBody),
		},
		{
			name: "bad",
			req:  mustRequest(badBody),
			err:  "failed to read request body: xxx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if cont := testErrCheck(t, "Sign()", tt.err, Sign(slackExampleSecret, tt.req)); !cont {
				return
			}

			ts := tt.req.Header.Get("X-Slack-Request-Timestamp")
			if len(ts) == 0 {
				t.Fatal("X-Slack-Request-Timestamp not set")
			}

			b, err := ioutil.ReadAll(tt.req.Body)
			testErrCheck(t, "ioutil.ReadAll()", "", err)

			s := fmt.Sprintf("v0:%s:%s", ts, b)

			got, want := tgenHMAC(t, s), tt.req.Header.Get("X-Slack-Signature")

			if got != want {
				t.Errorf("got = %q, want %q", got, want)
			}
		})
	}

	b := "{}"

	r, err := http.NewRequest(http.MethodPost, "http://example.org", strings.NewReader(b))
	testErrCheck(t, "http.NewRequest()", "", err)
	testErrCheck(t, "Sign()", "", Sign(slackExampleSecret, r))

	ts := r.Header.Get("X-Slack-Request-Timestamp")
	if len(ts) == 0 {
		t.Fatal("X-Slack-Request-Timestamp not set")
	}

	s := fmt.Sprintf("v0:%s:%s", ts, b)

	got, want := tgenHMAC(t, s), r.Header.Get("X-Slack-Signature")

	if got != want {
		t.Errorf("got = %q, want %q", got, want)
	}
}
