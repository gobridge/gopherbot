package gateway

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/theckman/gopher2/signing"
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

func slackSignatureMiddlewareFactory(hmacKey, token, appID, teamID string, baseLogger *zerolog.Logger, next http.HandlerFunc) http.HandlerFunc {
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

		r.Body = ioutil.NopCloser(bytes.NewReader(body))

		// validate that the signature looks good
		err = signing.Validate(hmacKey, signing.Request{
			Body:      body,
			Timestamp: r.Header.Get(signing.SlackTimestampHeader),
			Signature: r.Header.Get(signing.SlackSignatureHeader),
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

		rToken, err := getJSONString(document, "token")
		if err != nil {
			logger.Error().
				Err(err).
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if rToken != token {
			logger.Error().
				Str("error", "mismatched token").
				Str("token", rToken).
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		typeValue, err := getJSONString(document, "type")
		if err != nil {
			logger.Error().
				Err(err).
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// the following items will NOT be present
		// so let's skip them
		if typeValue == "url_verification" {
			next(w, r)
			return
		}

		rAppID, err := getJSONString(document, "api_app_id")
		if err != nil {
			logger.Error().
				Err(err).
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if rAppID != appID {
			logger.Error().
				Str("error", "mismatched api_app_id").
				Str("api_app_id", rAppID).
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rTeamID, err := getJSONString(document, "team_id")
		if err != nil {
			logger.Error().
				Err(err).
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
		}

		if rTeamID != teamID {
			logger.Error().
				Str("error", "mismatched team_id").
				Str("team_id", rTeamID).
				Msg("failed to validate Slack request")

			w.WriteHeader(http.StatusBadRequest)
			return
		}

		next(w, r)
	}
}
