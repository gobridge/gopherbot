package gotime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// Store represents the shape of the storage system.
type Store interface {
	Get(ctx context.Context) (id int64, notFound bool, err error)
	Put(ctx context.Context, lastID int64) error
}

// NotifyFunc represents the function signature the poller notifies on a new
// item. If error is not nil, the item will be retried at some point in the
// future.
type NotifyFunc func(context.Context) error

// GoTime tracks when it's Go Time!
type GoTime struct {
	logger            zerolog.Logger
	store             Store
	http              *http.Client
	notify            NotifyFunc
	startTimeVariance time.Duration

	lastNotified time.Time
}

// New constructs a *GoTime.
//
// startTimeVariance sets the window around the stream's start time when
// a live steam will be considered a GoTime live stream. This is necessary
// because the current changelog APIs return whether any show is streaming
// rather than GoTime specifically.
//
// notify is called when streaming starts. notify should return true when successful.
func New(s Store, c *http.Client, logger zerolog.Logger, startTimeVariance time.Duration, notify NotifyFunc) (*GoTime, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lastTS, notFound, err := s.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get last notitied timestamp: %w", err)
	}

	if notFound {
		// doing this explicitly to make sure we are good
		lastTS = 0

		if err = s.Put(ctx, lastTS); err != nil {
			return nil, fmt.Errorf("failed to initialize redis: %w", err)
		}
	}

	t := time.Unix(unix(lastTS))

	return &GoTime{
		logger:            logger,
		store:             s,
		http:              c,
		notify:            notify,
		startTimeVariance: startTimeVariance,
		lastNotified:      t,
	}, nil
}

func unix(i int64) (int64, int64) {
	// convert milliseconds to whole seconds
	// convert millisecond remainder from above conversion to nanoseconds
	return i / 1000, (i % 1000) * int64(time.Millisecond)
}

// Poll conditionally calls notify if GoTime is currently streaming.
//
// For a notification to be posted: a notification must not have been successful
// in the last 24 hours, changelog is currently streaming, and there is
// a GoTime episode scheduled within +/-startTimeVariance.
func (gt *GoTime) Poll(ctx context.Context) error {
	now := time.Now()
	if gt.lastNotified.After(now.Add(-24 * time.Hour)) {
		return nil
	}

	var status struct {
		Streaming bool
	}
	err := gt.get(ctx, "https://changelog.com/live/status", &status)
	if err != nil {
		return err
	}

	if !status.Streaming {
		return nil
	}

	var countdown struct {
		Data time.Time
	}
	err = gt.get(ctx, "https://changelog.com/slack/countdown/gotime", &countdown)
	if err != nil {
		return err
	}

	nextScheduled := countdown.Data
	if now.Before(nextScheduled.Add(-gt.startTimeVariance)) || now.After(nextScheduled.Add(gt.startTimeVariance)) {
		return nil
	}

	gt.logger.Trace().
		Msg("sending notification that it's Go Time")

	if err := gt.notify(ctx); err != nil {
		return fmt.Errorf("failed notification for Go Time: %w", err)
	}

	gt.lastNotified = now

	if err := gt.store.Put(ctx, now.UnixNano()/int64(time.Millisecond)); err != nil {
		return fmt.Errorf("failed to persist timestamp to redis;: %w", err)
	}

	return nil
}

// get makes an HTTP request to url and unmarshals the JSON response into i.
func (gt *GoTime) get(ctx context.Context, url string, i interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	req = req.WithContext(ctx)

	resp, err := gt.http.Do(req)
	if err != nil {
		return fmt.Errorf("making http request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

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
