package gotimestatus

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/rs/zerolog"
)

// timeNow returns the current time
// it is unexported and overridable to facilitate testing
var timeNow = time.Now

// Store represents the shape of the storage system.
type Store interface {
	Get(ctx context.Context) (id string, notFound bool, err error)
	Put(ctx context.Context, lastID string) error
}

// NotifyFunc represents the function signature the poller notifies on a new
// social status. If error is not nil, the item will be retried at some point in the
// future.
type NotifyFunc func(ctx context.Context, statusURL string) error

// GoTimeStatus posts social status updates from @gotime@changelog.social
type GoTimeStatus struct {
	logger       zerolog.Logger
	store        Store
	http         *http.Client
	notify       NotifyFunc
	statusMaxAge time.Duration

	lastStatus string
}

// New constructs a *GoTimeStatus.
//
// statusMaxAge sets the max age of a status to notify on
func New(s Store, c *http.Client, logger zerolog.Logger, statusMaxAge time.Duration, notify NotifyFunc) (*GoTimeStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lastStatus, notFound, err := s.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get last notitied timestamp: %w", err)
	}

	if notFound {
		// doing this explicitly to make sure we are good
		lastStatus = ""

		if err = s.Put(ctx, lastStatus); err != nil {
			return nil, fmt.Errorf("failed to initialize redis: %w", err)
		}
	}

	return &GoTimeStatus{
		logger:       logger,
		store:        s,
		http:         c,
		notify:       notify,
		statusMaxAge: statusMaxAge,
		lastStatus:   lastStatus,
	}, nil
}

type mastodonStatus struct {
	ID        string     `json:"id"`
	URL       string     `json:"url"`
	CreatedAt createTime `json:"created_at"`
}

type createTime struct {
	time.Time
}

func (ct *createTime) UnmarshalJSON(bs []byte) error {
	var str string
	if err := json.Unmarshal(bs, &str); err != nil {
		return err
	}
	t, err := time.Parse("2006-01-02T15:04:05.999Z", str)
	if err != nil {
		return nil
	}
	ct.Time = t
	return nil
}

const (
	// @gotime@changelog.social
	// acquired from curl 'https://changelog.social/api/v1/accounts/lookup?acct=gotime'
	// It's immutable so we don't need to look it up again
	gotimeAccountID = "109349735213354404"

	gotimeStatusesAPI = "https://changelog.social/api/v1/accounts/" + gotimeAccountID + "/statuses"
)

// Poll conditionally calls notify if there is a status update from @gotime@changelog.social
//
// For a status to be posted, it needs to be younger than the maxStatusAge
// this prevents very old statuses from being notified and acts as a safeguard
// if the last status ID could not be persisted to state storage, and prevents reposts if we lose the last status ID.
func (gt *GoTimeStatus) Poll(ctx context.Context) error {
	gt.logger.Trace().Msg("gotime status poll")
	now := timeNow()
	accountStatusURL := gotimeStatusesAPI
	if gt.lastStatus == "" {
		gt.logger.Trace().Msg("getting latest status")
	} else {
		gt.logger.Trace().Msgf("getting statuses since %s", gt.lastStatus)
		accountStatusURL = fmt.Sprintf("%s?since_id=%s", gotimeStatusesAPI, url.QueryEscape(gt.lastStatus))
	}
	var statuses []mastodonStatus
	err := gt.get(ctx, accountStatusURL, &statuses)
	if err != nil {
		return err
	}
	if len(statuses) == 0 {
		// No new statuses
		gt.logger.Trace().Msg("no statuses found")
		return nil
	}
	if gt.lastStatus == "" {
		// no last status, only notify on the latest status
		// which should be the first element in the list
		statuses = statuses[0:1]
	} else {
		// Sort statuses by age in ascending order
		sort.Slice(statuses, func(i, j int) bool {
			return statuses[i].CreatedAt.Time.Before(statuses[j].CreatedAt.Time)
		})
	}

	for _, status := range statuses {
		gt.lastStatus = status.ID
		age := now.Sub(status.CreatedAt.Time)
		if age > gt.statusMaxAge { // too old
			gt.logger.Trace().Msgf("status %s skipped. too old: %s", status.ID, age)
			continue
		}
		gt.logger.Trace().Msgf("notify gotime statuses: %s", status.URL)
		if err := gt.notify(ctx, status.URL); err != nil {
			return fmt.Errorf("failed to notify social status %s: %w", status.URL, err)
		}
	}
	if err := gt.store.Put(ctx, gt.lastStatus); err != nil {
		return fmt.Errorf("failed to persist status ID to redis: %w", err)
	}
	return nil
}

// get makes an HTTP request to url and unmarshals the JSON response into i.
func (gt *GoTimeStatus) get(ctx context.Context, url string, i interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	req = req.WithContext(ctx)

	resp, err := gt.http.Do(req)
	if err != nil {
		return fmt.Errorf("making http request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non-200 status code: %d - %s", resp.StatusCode, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	err = json.Unmarshal(body, i)
	if err != nil {
		return fmt.Errorf("unmarshaling response: %s", err)
	}

	return nil
}
