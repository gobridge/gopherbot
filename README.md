# gopherbot

This is the Go Slack Workspace bot. It uses the Slack Events API instead of the antiquated RTM (WebSocket)
API.

The bot will not respond to any messages more than 30 seconds ago.
If Slack experiences an issue preventing it from replying, the message will be dropped.

## Contributing

### Adding Responses / Reactions

All responses and reactions should be configured in the
[cmd/consumer/](https://github.com/gobridge/gopherbot/tree/main/cmd/consumer)
directory.

Each action should be split out by file.

For examples on how to configure and the use the various parts of the bot,
check out the [handler](handler) [package documentation](https://pkg.go.dev/github.com/gobridge/gopherbot/handler).

### Adding Definitions to Glossary

There is also the `define` command that is powered by the `glossary` package. If
you'd like to add definitions to the glossary, you can [do it here](https://github.com/gobridge/gopherbot/blob/main/glossary/terms.go#L5)
and raise a PR against this repo.

The glossary is meant to contain common words and terms relevant to the Go
community. It's not Urban Dictionary.

## Architecture

### Slack API

As mentioned above, the bot uses [The Slack Events API](https://api.slack.com/events-api).

It's a HTTP+JSON based subscription model, with strict requirements on message
acknowledgment times on delivery. Based on that, the best way to accept events
is to write them to a queue to be processed by workers later so that some slow
task doesn't break the contract or introduce the risk of lossy message processing.

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

The [bgtasks](cmd/bgtasks) component is meant to be a place where regular background
jobs run, such as filling data caches, polling for Gerrit (Go CL) merges, or GoTime
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

## Deployment

The bot is currently running under the GoBridge Heroku organization.

Merges to `main` are automatically deployed to the staging version `@glenda`**.
If a  merge to `main` was deployed successfully, you need to go into
the Heroku UI and promote each running app to production.

**Please Note:** When clicking the `Promote to Production` button,
you need to deselect the unrelated apps so that you don't accidentally
promote the wrong build to production.

For example, if you're promoting the gateway component you need to make
sure not to promote it to the `bgtasks` or `consumer` apps.
This will break the bot, and require some manual action to fix the
production deployment.
