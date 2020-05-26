package gerrit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

const gerritURL = "https://go-review.googlesource.com/changes/?q=status:merged&O=12&n=100"

// CL represents a merged CL that we send to the subscriber of the poller.
type CL struct {
	Project         string `json:"project"`
	ChangeID        string `json:"change_id"`
	Number          int64  `json:"_number"`
	Subject         string `json:"subject"`
	Branch          string `json:"branch"`
	CurrentRevision string `json:"current_revision"`
	Revisions       map[string]struct {
		Commit struct {
			Subject string `json:"subject"`
			Message string `json:"message"`
		} `json:"commit"`
	} `json:"revisions"`
}

// Link returns the golang.org CL link for this CL, in the form of
// https://golang.org/cl/<Number>/
func (cl *CL) Link() string {
	return fmt.Sprintf("https://golang.org/cl/%d/", cl.Number)
}

// Message returns a string suitable as a subject line in a message about the
// CL.
func (cl *CL) Message() string {
	subject := cl.Subject
	if cl.Project != "go" {
		subject = fmt.Sprintf("[%s] %s", cl.Project, subject)
	}

	return subject
}

// NotifyFunc represents the function signature the poller notifies on a new
// item. If error is not nil, the item will be retried at some point in the
// future.
type NotifyFunc func(context.Context, CL) error

// Store represents the shape of the storage system.
type Store interface {
	Get(ctx context.Context) (id int64, notFound bool, err error)
	Put(ctx context.Context, lastID int64) error
}

// Gerrit tracks merged CLs.
type Gerrit struct {
	store  Store
	http   *http.Client
	logger zerolog.Logger
	notify NotifyFunc

	lastID int64
}

// ErrNotFound should be returned by Store implementations when CL number
// doesn't exist.
var ErrNotFound = errors.New("CL not found")

// New creates an initializes an instance of Gerrit.
func New(s Store, http *http.Client, logger zerolog.Logger, notify NotifyFunc) (*Gerrit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	lastID, notFound, err := s.Get(ctx)

	cancel()

	if err != nil {
		return nil, fmt.Errorf("failed to get latest ID: %w", err)
	}

	if notFound {
		lastID = -1
	}

	return &Gerrit{
		store:  s,
		http:   http,
		logger: logger,
		notify: notify,
		lastID: lastID,
	}, nil
}

// Poll checks for new merged CLs and calls notify for each CL.
func (g *Gerrit) Poll(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", gerritURL, nil)
	if err != nil {
		return err
	}
	req.Header.Add("User-Agent", "Gophers Slack bot")
	req = req.WithContext(ctx)

	resp, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get data from Gerrit: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got non-200 code: %d from gerrit api", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	// Gerrit prefixes responses with `)]}'`
	// https://gerrit-review.googlesource.com/Documentation/rest-api.html#output
	body = bytes.TrimPrefix(body, []byte(")]}'"))

	var cls []CL
	err = json.Unmarshal(body, &cls)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON body: %w", err)
	}

	// if g.lastID is -1, we have zero state
	// so grab only the latest CL and use that one
	if g.lastID == -1 && len(cls) > 0 {
		cls = cls[:1]

		g.logger.Info().
			Int64("last_id", cls[0].Number).
			Msg("initializing last CL ID to latest")

	} else {
		// The change output is sorted by the last update time, most recently updated to oldest updated.
		// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#list-changes
		for i, cl := range cls {
			if cl.Number == g.lastID {
				cls = cls[:i]
				break
			}
		}
	}

	for i := len(cls) - 1; i >= 0; i-- {
		cl := cls[i]

		g.logger.Trace().
			Int64("cl_num", cl.Number).
			Msg("sending notification of CL merged")

		if err = g.notify(ctx, cl); err != nil {
			return fmt.Errorf("notification failed: %w", err)
		}

		if err = g.store.Put(ctx, cl.Number); err != nil {
			return fmt.Errorf("failed to persist CL %d: %w", cl.Number, err)
		}

		g.lastID = cl.Number
	}

	return nil
}
