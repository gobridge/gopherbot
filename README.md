# gopherbot

This is a rewrite of the [original](https://github.com/gobridge/gopher) Go Slack
Workspace chat bot. This big difference between that version and this, is that
`gopherbot` uses the Slack Events API instead of the antiquated RTM (WebSocket)
API.

This unfortunately results in a more complicated queue-based architecture, that
results in a more resilient chat bot. As a result of it now being a queue-based
system, the bot will not respond to any messages more than 30 seconds ago. So if
Slack experiences an issue preventing us from replying, the message will be
dropped.

## Contributing
### Adding Responses / Reactions
Pretty much all responses and reactions should be configured in the
[cmd/consumer/](https://github.com/gobridge/gopherbot/tree/master/cmd/consumer)
directory, with each thing being split out by file. How to configure each should
be fairly straightforward based on existing examples, and the usage of the
`handler` package is documented via GoDoc if you have any questions.

### Adding Definitions to Glossary
There is also the `define` command that is powered by the `glossary` package. If
you'd like to add definitions to the glossary, you can [do it
here](https://github.com/gobridge/gopherbot/blob/master/glossary/terms.go#L5)
and raise a PR against this repo.

The glossary is meant to contain common words and terms relevant to the Go
community. It's not Urban Dictionary.

## Architecture
### Slack API
As mentioned above, the old version used the RTM API for interacting with Slack.
This is no longer the recommended API to use for building integrations, with
them now suggesting [The Slack Events API](https://api.slack.com/events-api).

It's a HTTP+JSON based subscription model, with strict requirements on message
acknowledgment times on delivery. Based on that, the best way to accept events
is to write them to a queue to be processed by workers later so that some slow
task doesn't violate the contract or introduce the risk of lossy message
processing.

The Events API offers signing of requests, so that you can be confident the
request originated from Slack.

### Components
#### Gateway
The job for the gateway is to cryptographically validate the incoming event from
Slack, confirm that it contains the metadata we expect, and then forward the
message on to the work queue.

This is a pretty simple gateway, although it does use `fastjson` to avoid
reflection to make queue routing logic decisions (based on JSON event type).
There is effectively one queue for event type:

- messages (private vs public)
- new users joining workspace
- new users joining a channel

The gateway is stateless and can be scaled horizontally.

#### Consumer
The consumer registers a handler for each of the queues, and those handlers
process each message internally. They themselves may have sub-handlers that get
executed, like reacting to messages with emoji versus responding to them.

If you're looking to add commands, reactions, a channel join message, or an
update to the workspace join message this is the component that handles those.

The consumer is stateless and can be scaled horizontally.

#### BGTasks
The `bgtasks` component is meant to be a place where regular background jobs are
ran, such as filling data caches, polling for Gerrit (Go CL) merges, or GoTime
shows starting. 

This currently has a channel cache poller, so that consumer handlers can look up
channels by name without making many Slack API calls.

Things here cannot be safely scaled horizontally, as it could cause double
messages or excessive API calls / cache fills. These jobs are kept here so that
we can avoid dealing with cluster locking, in addition to our work queue. :)

#### Redis
More specifically, Heroku Redis. We use Redis Streams to implement the bot's
workqueue. It's also where we cache some data for use in the handlers, such as
mapping channel names to IDs.

## Local Development
Let us get back to you on this one. :)

The most straightforward way is to run it in Heroku yourself, or simulate the
environment with these environment variables:

| Environment Var                 | Description                                                                                                                                             |
| :--------------                 | -----------                                                                                                                                             |
| `PORT`                          | The port to bind to, used for the `gateway` component.                                                                                                  |
| `REDIS_URL`                     | Expects this in the format the Heroku provides it: `redis://u:pass@host:port`.                                                                          |
| `GOPHER_REDIS_INSECURE`         | Set to `1` if Redis is over an insecure connection.                                                                                                     |
| `GOPHER_REDIS_SKIPVERIFY`       | Set to `1` if you want Redis client to not verify TLS connection. Heroku Redis's certificate cannot be validated, so tis is required for production. :( |
| `GOPHER_LOG_LEVEL`              | Any level as recognized by [github.com/rs/zerolog](https://github.com/rs/zerolog).                                                                      |
| `GOPHER_SLACK_APP_ID`           | The App's unique ID. Starts with `A`.                                                                                                                   |
| `GOPHER_SLACK_TEAM_ID`          | The installed workspace's unique ID. Starts with `T`.                                                                                                   |
| `GOPHER_SLACK_CLIENT_ID`        | The OAuth Client ID. Currently unused.                                                                                                                  |
| `GOPHER_SLACK_CLIENT_SECRET`    | The OAuth Client secret. Currently unused.                                                                                                              |
| `GOPHER_SLACK_REQUEST_TOKEN`    | This is the static Verification Token in the App's configuration pane, sent with every request.                                                         |
| `GOPHER_SLACK_REQUEST_SECRET`   | This is the called the Signing Secret in the App's configuration pane, used to cryptographically validate the request.                                  |
| `GOPHER_SLACK_BOT_ACCESS_TOKEN` | The Slack API token for the Bot App. Starts with `xoxb-`.                                                                                               |
| `HEROKU_APP_ID`                 | The UUID Heroku has given to the application. This should be set.                                                                                       |
| `HEROKU_APP_NAME`               | The human-readable name of the application. This is used for Redis key generation, and must be set.                                                     |
| `HEROKU_DYNO_ID`                | The UUID Heroku gives each Dyno (worker process). This is used for Redis key generation, and must be set.                                               |
| `HEROKU_SLUG_COMMIT`            | The commit of the code running. This is used in logging, and should be set.                                                                             |
