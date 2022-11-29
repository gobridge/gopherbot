package gotimestatus

import (
	"bytes"
	"context"
	"github.com/rs/zerolog"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestGoTimeStatus_Poll(t *testing.T) {
	t.Cleanup(func() {
		timeNow = time.Now
	})
	timeNow = func() time.Time {
		now, _ := time.Parse(time.RFC3339, "2022-11-24T15:20:00Z")
		return now
	}
	zl := zerolog.New(ioutil.Discard)
	s := make(mockStore)
	c := &http.Client{
		Transport: &mockResponseTransport{response: testData(t, "statuses.json")},
	}
	const (
		expectedURL      = "https://changelog.social/@gotime/109399448077436200"
		expectedStatusID = "109399448077436200"
	)
	var notifyURL string
	gts, err := New(s, c, zl, 5*time.Minute, func(ctx context.Context, statusURL string) error {
		notifyURL = statusURL
		return nil
	})
	if err != nil {
		return
	}
	if err := gts.Poll(context.Background()); err != nil {
		t.Fatalf("unexpected poll error: %v", err)
	}
	if notifyURL != expectedURL {
		t.Errorf("status URL: expected %s, got %s", expectedURL, notifyURL)
	}
	if gts.lastStatus != expectedStatusID {
		t.Errorf("lastStatus: expected %s, got %s", expectedStatusID, gts.lastStatus)
	}
	if v, ok := s[redisKey]; !ok || v != expectedStatusID {
		t.Errorf("store: expected (%s,true), got (%s,%t)", expectedStatusID, v, ok)
	}
}

func TestGoTimeStatus_Poll_lastID(t *testing.T) {
	t.Cleanup(func() {
		timeNow = time.Now
	})
	timeNow = func() time.Time {
		now, _ := time.Parse(time.RFC3339, "2022-11-24T15:20:00Z")
		return now
	}
	zl := zerolog.New(ioutil.Discard)
	s := make(mockStore)
	c := &http.Client{
		Transport: &mockResponseTransport{response: testData(t, "statuses_since.json")},
	}
	const (
		expectedURL      = "https://changelog.social/@gotime/109399448077436200"
		expectedStatusID = "109399448077436200"
	)
	var notifyURL string
	gts, err := New(s, c, zl, 5*time.Minute, func(ctx context.Context, statusURL string) error {
		notifyURL = statusURL
		return nil
	})
	gts.lastStatus = "109378535144130594" // Set last status to test skipping old messages
	if err != nil {
		return
	}
	if err := gts.Poll(context.Background()); err != nil {
		t.Fatalf("unexpected poll error: %v", err)
	}
	if notifyURL != expectedURL {
		t.Errorf("status URL: expected %s, got %s", expectedURL, notifyURL)
	}
	if gts.lastStatus != expectedStatusID {
		t.Errorf("lastStatus: expected %s, got %s", expectedStatusID, gts.lastStatus)
	}
	if v, ok := s[redisKey]; !ok || v != expectedStatusID {
		t.Errorf("store: expected (%s,true), got (%s,%t)", expectedStatusID, v, ok)
	}
}

type mockResponseTransport struct {
	response []byte
}

type mockStore map[string]string

func (m mockStore) Get(ctx context.Context) (id string, notFound bool, err error) {
	v, ok := m["last_id"]
	if !ok {
		return "", true, nil
	}
	return v, false, nil
}

func (m mockStore) Put(ctx context.Context, lastID string) error {
	m["last_id"] = lastID
	return nil
}

var _ Store = mockStore{}

func (m *mockResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	rr := httptest.NewRecorder()
	rr.Body = bytes.NewBuffer(m.response)
	rr.Code = http.StatusOK
	return rr.Result(), nil
}

var _ http.RoundTripper = &mockResponseTransport{}

func testData(t *testing.T, name string) []byte {
	fp := filepath.Join("testdata", name)
	data, err := ioutil.ReadFile(fp)
	if err != nil {
		t.Fatalf("could not read %s: %v", fp, err)
	}
	return data
}
