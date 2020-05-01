package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/theckman/gopher2/signing"
	"github.com/theckman/gopher2/workqueue"
	"github.com/valyala/fastjson"
)

type ctxKey uint8

const (
	ctxKeyReqID ctxKey = iota
)

func ctxRequestID(ctx context.Context) (string, bool) {
	v := ctx.Value(ctxKeyReqID)
	if v == nil {
		return "", false
	}

	rid, ok := v.(string)
	if !ok {
		return "", false
	}

	return rid, true
}

func (s *server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}

func (s *server) handleRUOK(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "imok")
}

func chMiddlewareFactory(baseLogger zerolog.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()

		if rid := r.Header.Get("X-Request-ID"); len(rid) > 0 {
			ctx = context.WithValue(ctx, ctxKeyReqID, rid)
			w.Header().Set("X-Request-ID", rid)
		}

		// Slack expects a response within 3 seconds, give ourselves 2.9 seconds
		ctx, cancel := context.WithTimeout(ctx, 2900*time.Millisecond)

		defer cancel()

		next(w, r.WithContext(ctx))
	}
}

const (
	// TODO(theckman): make this a parameter?
	slackTeamID = "T029RQSE6"

	maxBodySize = 2 * 1024 * 1024 // 2 MB
)

func slackSignatureMiddlewareFactory(hmacKey, token, appID string, baseLogger *zerolog.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lc := baseLogger.With()

		rid, _ := ctxRequestID(r.Context())
		lc = lc.Str("request_id", rid)

		logger := lc.Str("context", "slack_middleware").Logger()

		body, err := ioutil.ReadAll(io.LimitReader(r.Body, maxBodySize))
		if err != nil {
			logger.Error().
				Err(err).
				Msg("failed to read request body")

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// validate that the signature looks good
		err = signing.Validate(hmacKey, signing.Request{
			B: body,
			T: r.Header.Get(signing.SlackTimestampHeader),
			S: r.Header.Get(signing.SlackSignatureHeader),
		})
		if err != nil {
			logger.Error().
				Err(err).
				Msg("failed to validated Slack request")

			w.WriteHeader(http.StatusBadRequest)
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

		if !document.Exists("token") {
			logger.Error().
				Str("error", "missing token field").
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rToken := document.Get("token")
		if rt := rToken.String(); rt != token {
			logger.Error().
				Str("error", "mismatched token").
				Str("token", rt).
				Str("expected", token).
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !document.Exists("api_app_id") {
			logger.Error().
				Str("error", "missing api_app_id field").
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rAppID := document.Get("api_app_id")
		if rai := rAppID.String(); rai != appID {
			logger.Error().
				Str("error", "mismatched api_app_id").
				Str("api_app_id", rai).
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !document.Exists("type") {
			logger.Error().
				Str("error", "missing type field").
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		typeValue := document.Get("type")
		if typ := typeValue.Type(); typ != fastjson.TypeString {
			logger.Error().
				Str("error", "type field is not a string").
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return

		}

		if typeValue.String() == "url_verification" {
			next(w, r)
			return
		}

		if !document.Exists("team_id") {
			logger.Error().
				Str("error", "missing team_id field").
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rTeamID := document.Get("team_id")
		if rti := rTeamID.String(); rti != slackTeamID {
			logger.Error().
				Str("error", "mismatched team_id").
				Str("team_id", rti).
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		r.Body = ioutil.NopCloser(bytes.NewReader(body))
		next(w, r)
	}
}

func requestValues(document *fastjson.Value) (eventType, eventID string, eventTime int64, err error) {
	if !document.Exists("type") {
		return "", "", 0, errors.New("type field missing")
	}

	v := document.Get("type")
	if typ := v.Type(); typ != fastjson.TypeString {
		return "", "", 0, fmt.Errorf("type field is a %s, want a string", typ)
	}

	eventType = v.String()

	if eventType == "url_verification" {
		return
	}

	if !document.Exists("event_id") {
		return "", "", 0, errors.New("event_id field missing")
	}

	if !document.Exists("event_id") {
		return "", "", 0, errors.New("event_id field missing")
	}

	if !document.Exists("event_time") {
		return "", "", 0, errors.New("event_time field missing")
	}

	if !document.Exists("event") {
		return "", "", 0, errors.New("event field mising")
	}

	v = document.Get("event_id")
	if typ := v.Type(); typ != fastjson.TypeString {
		return "", "", 0, fmt.Errorf("event_id field is a %s, want a string", typ)
	}

	eventID = v.String()

	v = document.Get("event_time")
	if typ := v.Type(); typ != fastjson.TypeNumber {
		return "", "", 0, fmt.Errorf("event_id field is a %s, want a number", typ)
	}

	eventTime, err = v.Int64()
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to parse event_time: %w", err)
	}

	return
}

func urlVerification(w http.ResponseWriter, r *http.Request, document *fastjson.Value, logger zerolog.Logger) {
	if !document.Exists("challenge") {
		logger.Error().
			Str("error", "challenge field missing").
			Msg("failed URL verification")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	challenge := document.Get("challenge").String()

	w.Header().Set("Content-Type", "plain/text")
	fmt.Fprint(w, challenge)
}

func wqEventType(event *fastjson.Value) (workqueue.Event, error) {
	if !event.Exists("type") {
		return "", errors.New("type field not present in event")
	}

	switch t := event.Get("type").String(); t {
	case "message":
		if !event.Exists("channel_type") {
			return workqueue.SlackMessageChannel, nil
		}

		ct := event.Get("channel_type").String()

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
	default:
		return "", fmt.Errorf("unknown type %s", t)
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

	logger.Debug().
		Str("event_type", string(et)).
		Int64("event_timestamp", eventTimestamp).
		Str("event_id", eventID).
		Bool("object_has_len", len(object) > 0).
		Msg("would publish event")

	/*
		err = s.q.Publish(et, eventTimestamp, eventID, rid, object)
		if err != nil {
			logger.Error().Err(err).Msg("failed to publish event to workqueue")
			w.WriteHeader(http.StatusInternalServerError)

		}
	*/
}
