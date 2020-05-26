// Package handler providers handler/router mechanisms for specific actions or
// event types.
package handler

import (
	"math/rand"
	"time"

	"github.com/slack-go/slack"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// ChannelCache is the interface to describe the shape of a channel cache we
// accept.
type ChannelCache interface {
	Lookup(name string) (slack.Channel, bool)
}
