// Package playground provides a handler.MatchFn and a Client struct with a
// Handler method that can be used as handler.ActionFn.
package playground

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gobridge/gopherbot/handler"
	"github.com/gobridge/gopherbot/mparser"
	"github.com/gobridge/gopherbot/workqueue"
	"github.com/rs/zerolog"
)

// Client is the Go Playground client.
type Client struct {
	httpc     *http.Client
	logger    zerolog.Logger
	blacklist map[string]struct{}
}

// New takes an HTTP client and returns a Playground Client. If httpc is nil
// this program will probably panic at some point.
func New(httpc *http.Client, logger zerolog.Logger, channelBlacklist []string) *Client {
	m := make(map[string]struct{}, len(channelBlacklist))

	for _, cid := range channelBlacklist {
		m[cid] = struct{}{}
	}

	return &Client{
		httpc:     httpc,
		logger:    logger,
		blacklist: m,
	}
}

// Handler is a handler.ActionFn.
func (c *Client) Handler(ctx workqueue.Context, m handler.Messenger, r handler.Responder) error {
	for _, file := range m.Files() {
		if file.Filetype == "go" || file.Filetype == "text" {
			return c.pgForFiles(ctx, m, r)
		}
	}

	return c.pgForMessage(ctx, m, r)
}

func (c *Client) pgForMessage(ctx workqueue.Context, m handler.Messenger, r handler.Responder) error {
	link, err := c.upload(ctx, strings.NewReader(m.Text()))
	if err != nil {
		return fmt.Errorf("failed to upload to playground: %w", err)
	}

	mention := mparser.Mention{
		Type: mparser.TypeUser,
		ID:   m.UserID(),
	}

	msg := fmt.Sprintf("The above code from %s in the playground: <%s>", mention.String(), link)

	err = r.Respond(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to send message with Playground link: %w", err)
	}

	err = r.RespondEphemeral(ctx, `I've noticed you've written a large block of text (more than 9 lines). `+
		`To faciliate collaboration and make the conversation easier to follow, `+
		`please consider using <https://play.golang.org> to share code. If you wish to not `+
		`link against the playground, please start the message with "nolink". Thank you!`,
	)
	if err != nil {
		ctx.Logger().Error().
			Err(err).
			Msg("failed to respond with ephemeral Go Playground etiquette")
	}

	return nil
}

func (c *Client) pgForFiles(ctx workqueue.Context, m handler.Messenger, r handler.Responder) error {
	sc := ctx.Slack()
	files := m.Files()

	// XXX(theckman): following comment and code has been copied verbatim from gopherv1
	//
	// Empirically, attempting to call GetFileInfoContext too quickly after a
	// file is uploaded can cause a "file_not_found" error.
	time.Sleep(1 * time.Second)

	for _, f := range files {
		i, _, _, err := sc.GetFileInfoContext(ctx, f.ID, 0, 0)
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %w", f.ID, err)
		}

		if i.Lines < 6 || i.PrettyType == "Plain Text" {
			return nil
		}

		buf := &bytes.Buffer{}
		err = sc.GetFile(i.URLPrivateDownload, buf)
		if err != nil {
			return fmt.Errorf("failed to get file %s: %w", f.ID, err)
		}

		link, err := c.upload(ctx, buf)
		if err != nil {
			return fmt.Errorf("failed to upload to playground: %w", err)
		}

		mention := mparser.Mention{
			Type: mparser.TypeUser,
			ID:   m.UserID(),
		}

		msg := fmt.Sprintf("The above code from %s in the playground: <%s>", mention.String(), link)
		err = r.Respond(ctx, msg)
		if err != nil {
			return fmt.Errorf("failed to send message with Playground link: %w", err)
		}
	}

	err := r.RespondEphemeral(ctx, `I've noticed you uploaded a Go file. To facilitate collaboration and make `+
		`it easier for others to share back the snippet, please consider using: `+
		`<https://play.golang.org>. If you wish to not link against the playground, please use `+
		`"nolink" in the message. Thank you!`,
	)
	if err != nil {
		ctx.Logger().Error().
			Err(err).
			Msg("failed to respond with ephemeral Go Playground etiquette")
	}

	return nil
}

func (c *Client) upload(ctx context.Context, body io.Reader) (link string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://play.golang.org/share", body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Add("User-Agent", "Gophers Slack Bot V2")

	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected HTTP response status: %s", resp.Status)
	}

	id, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return "https://play.golang.org/p/" + string(id), nil
}

// MessageMatchFn satisfies handler.MessageMatchFn
func (c *Client) MessageMatchFn(shadowMode bool, m handler.Messenger) bool {
	// channel is blacklisted
	if _, ok := c.blacklist[m.ChannelID()]; ok {
		c.logger.Debug().
			Str("reason", "channel not permitted").
			Msg("playground match skipped")
		return false
	}

	rt := m.RawText()

	if strings.Contains(rt, "nolink") || (len(m.Files()) == 0 && strings.Count(rt, "\n") < 10) {
		return false
	}

	if shadowMode {
		c.logger.Debug().
			Str("reason", "shadow mode").
			Msg("playground match skipped")

		return false
	}

	return true
}
