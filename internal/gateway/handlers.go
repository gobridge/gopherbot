package gateway

import (
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/theckman/gopher2/workqueue"
	"github.com/valyala/fastjson"
)

const maxBodySize = 2 * 1024 * 1024 // 2 MB

func (s *server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}

func (s *server) handleRUOK(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "imok")
}

func getJSONString(document *fastjson.Value, key string) (string, error) {
	if !document.Exists(key) {
		return "", fmt.Errorf("failed to get field %s: key does not exist", key)
	}

	v, err := document.Get(key).StringBytes()
	if err != nil {
		return "", fmt.Errorf("failed to get field %s: %w", key, err)
	}

	s := make([]byte, len(v))

	copy(s, v)

	return string(s), nil
}

func getJSONInt64(document *fastjson.Value, key string) (int64, error) {
	if !document.Exists(key) {
		return -1, fmt.Errorf("failed to get field %s: key does not exist", key)
	}

	v, err := document.Get(key).Int64()
	if err != nil {
		return -1, fmt.Errorf("failed to get field %s: %w", key, err)
	}

	return v, nil
}

func requestValues(document *fastjson.Value) (eventType, eventID string, eventTimestamp int64, err error) {
	eventType, err = getJSONString(document, "type")
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get type field: %w", err)
	}

	if eventType == "url_verification" {
		return
	}

	eventID, err = getJSONString(document, "event_id")
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get event_id field")
	}

	eventTimestamp, err = getJSONInt64(document, "event_time")
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get event_time field: %w", err)
	}

	return
}

func urlVerification(w http.ResponseWriter, r *http.Request, document *fastjson.Value, logger zerolog.Logger) {
	challenge, err := getJSONString(document, "challenge")
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed URL verification")

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "plain/text")
	fmt.Fprint(w, challenge)
}

func wqEventType(event *fastjson.Value) (workqueue.Event, error) {
	eventType, err := getJSONString(event, "type")
	if err != nil {
		return "", fmt.Errorf("failed to get type field: %w", err)
	}

	switch eventType {
	case "message":
		if !event.Exists("channel_type") {
			return workqueue.SlackMessageChannel, nil
		}

		ct, _ := getJSONString(event, "channel_type")

		switch ct {
		case "app_home":
			return workqueue.SlackMessageAppHome, nil
		case "channel":
			return workqueue.SlackMessageChannel, nil
		case "group":
			return workqueue.SlackMessageGroup, nil
		case "im":
			return workqueue.SlackMessageIM, nil
		case "mpim":
			return workqueue.SlackMessageMPIM, nil
		default:
			return workqueue.SlackMessageChannel, nil
		}

	case "team_join":
		return workqueue.SlackTeamJoin, nil

	case "member_joined_channel":
		return workqueue.SlackChannelJoin, nil

	default:
		return "", fmt.Errorf("unknown type %s", eventType)
	}
}

func (s *server) handleSlackEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lc := s.l.With().Str("context", "event_handler")

	rid, ok := ctxRequestID(ctx)
	if ok {
		lc = lc.Str("request_id", rid)
	}

	logger := lc.Logger()

	if r.Method != http.MethodPost {
		logger.Info().
			Str("http_method", r.Method).
			Msg("unexpected HTTP method")

		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to parse Content-Type")

		w.Header().Set("Accept", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if mt != "application/json" {
		logger.Error().
			Str("content_type", mt).
			Msg("content type was not JSON")

		w.Header().Set("Accept", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to read request body")

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	document, err := fastjson.ParseBytes(body)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to unmarshal JSON document")

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	eventType, eventID, eventTimestamp, err := requestValues(document)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to parse values from JSON document")

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if eventType == "url_verification" {
		urlVerification(w, r, document, logger)
		return
	}

	logger = logger.With().Str("event_type", eventType).Str("event_id", eventID).Int64("event_time", eventTimestamp).Logger()

	if !document.Exists("event") {
		logger.Error().
			Str("error", "event field does not exist").
			Msg("failed to unmarshal JSON document")

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	event := document.Get("event")
	et, err := wqEventType(event)
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to determine event type")

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	obj, err := event.Object()
	if err != nil {
		logger.Error().
			Err(err).
			Msg("failed to convert event field to object")

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	object := obj.MarshalTo(make([]byte, 0, 4*1024))

	err = s.q.Publish(et, eventTimestamp, eventID, rid, object)
	if err != nil {
		logger.Error().Err(err).Msg("failed to publish event to workqueue")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Debug().
		Str("event_type", string(et)).
		Int64("event_timestamp", eventTimestamp).
		Str("event_id", eventID).
		Bool("object_has_len", len(object) > 0).
		Msg("published event")
}
