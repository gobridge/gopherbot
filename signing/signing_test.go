package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
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

func TestValidate(t *testing.T) {
	body := "testBody"
	now := strconv.Itoa(int(time.Now().Unix()))
	m := hmac.New(sha256.New, []byte(slackExampleSecret))

	// generate signature without body
	s := fmt.Sprintf("v0:%s:%s", now, "")
	_, err := m.Write([]byte(s))

	testErrCheck(t, "m.Write()", "", err)

	sigA := fmt.Sprintf("v0=%x", m.Sum(nil))

	m.Reset()

	// generate signature with the test body
	s = fmt.Sprintf("v0:%s:%s", now, body)
	_, err = m.Write([]byte(s))

	testErrCheck(t, "m.Write()", "", err)

	sigB := fmt.Sprintf("v0=%x", m.Sum(nil))

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
				T: "1531420618",
			},
			err: "X-Slack-Signature header not present",
		},
		{
			name: "garbage_timestamp",
			r: Request{
				T: "lol",
				S: "v0=55f41ec73231010289b54e669149ea021fccab11b5524355523533ce930cb739",
			},
			err: `failed to parse X-Slack-Request-Timestamp header: strconv.ParseInt: parsing "lol": invalid syntax`,
		},
		{
			name: "old_timestamp",
			r: Request{
				T: "1531420618",
				S: "v0=55f41ec73231010289b54e669149ea021fccab11b5524355523533ce930cb739",
			},
			err: "request timestamp (1531420618) too old",
		},
		{
			name: "ok_no_body",
			r: Request{
				T: now,
				S: sigA,
			},
		},
		{
			name: "ok_body",
			r: Request{
				T: now,
				S: sigB,
				B: []byte(body),
			},
		},
		{
			name: "wrong",
			r: Request{
				T: now,
				S: sigB + "x",
				B: []byte(body),
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
