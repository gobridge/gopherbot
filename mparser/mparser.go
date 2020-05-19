// Package mparser parses the text from a Slack message, identifying user,
// group, and channel mentions. This package also supports stripping those
// mentions from the message, to allow for easier command processing.
package mparser

import (
	"fmt"
	"strings"
)

// Type is a faux-enum for describing whether it was a user or group mention.
type Type uint8

const (
	// TypeUser represents a user mention.
	TypeUser Type = 1 << iota

	// TypeGroup represents a workspace group mention.
	TypeGroup

	// TypeHere is for @here mentions. If this Type is used, the
	// Mention.ID will be the channel's ID.
	TypeHere

	// TypeChannel is for @channel mentions. If this Type is used, the
	// Mention.ID will be the channel's ID.
	TypeChannel

	// TypeEveryone is for @everyone metnions. If this Type is used, the
	// Mention.ID will be the channel's ID.
	TypeEveryone

	// TypeChannelRef is a reference to another channel in a message, like if a
	// user were to type "join #general".
	TypeChannelRef
)

func (t Type) String() string {
	switch t {
	case TypeUser:
		return "user"
	case TypeGroup:
		return "group"
	case TypeHere:
		return "here"
	case TypeChannel:
		return "channel"
	case TypeEveryone:
		return "everyone"
	case TypeChannelRef:
		return "channelref"
	default:
		return "invalid"
	}
}

func typeFromStr(s string) Type {
	switch s {
	case "channel":
		return TypeChannel

	case "here":
		return TypeHere

	case "everyone":
		return TypeEveryone
	}

	return 100
}

// Mention represents a Slack message mention. This could be mentioning a user,
// a group, or special things like @here, @channel, and @everyone.
//
// If Type is TypeUser or TypeGroup, the ID field will be the ID of the user or
// group that was mentioned. If the Type is TypeHere, TypeChannel, or
// TypeEveryone, the ID is the channel the message was sent in.
//
// If the Type is TypeChannelRef, it's someone mentioning a channel in the
// message, and may include a Label. There is no guarantee this will be set.
type Mention struct {
	Type  Type
	ID    string
	Label string
}

func (m Mention) String() string {
	var prefix string
	switch m.Type {
	case TypeHere:
		return "<!here>"

	case TypeChannel:
		return "<!channel>"

	case TypeEveryone:
		return "<!everyone>"

	case TypeUser:
		prefix = "@"

	case TypeChannelRef:
		prefix = "#"

	case TypeGroup:
		prefix = "!subteam^"

	default:
		return "invalid"
	}

	return fmt.Sprintf("<%s%s>", prefix, m.ID)
}

// MarshalText satisfies the encoding.TextMarshaler interface.
func (m Mention) MarshalText() ([]byte, error) {
	switch m.Type {
	case TypeHere, TypeChannel, TypeEveryone, TypeUser, TypeChannelRef, TypeGroup:
		return []byte(m.String()), nil

	default:
		return nil, fmt.Errorf("unknown type value: %d", m.Type)
	}
}

// Join takes a slice of mentions, and joins them using the sep.
func Join(mentions []Mention, sep string) string {
	if len(mentions) == 0 {
		return ""
	}

	buf := &strings.Builder{}

	for _, m := range mentions {
		buf.WriteString(m.String())
		buf.WriteString(sep)
	}

	return buf.String()
}

// ParseAndSplice calls Parse(), and uses the start/end index of each mention to
// remove it from the message and return the resulting string. Please see the
// Parse() documentation for more information on parsing.
func ParseAndSplice(message, channelID string) (string, []Mention) {
	ms, ls := Parse(message, channelID)
	if len(ms) == 0 {
		return message, nil
	}

	m := []byte(message)
	b := &strings.Builder{}

	var start int

	for _, area := range ls {
		b.Write(m[start:area[0]])
		start = area[1] + 1
	}

	b.Write(m[start:])

	return b.String(), ms
}

type pmode uint8

const (
	pmodeInit pmode = iota
	pmodeOpen pmode = 1 << (iota - 1)
	pmodeAt
	pmodeEx
	pmodeHash
	pmodePipe
	pmodeUser
	pmodeGroup
)

// Parse takes the message text, and the channel ID where the message was sent,
// and parses the mentions in the message. This returns the list of mentions,
// and the start/end index of each mention to allow you to locate them.
//
// For @here, @channel, and @everyone the Mention.ID is set to the channelID.
func Parse(message, channelID string) ([]Mention, [][]int) {
	if strings.IndexByte(message, '<') == -1 {
		return nil, nil
	}

	var tmp string
	var mode pmode // pmodeInit
	var start int
	var mentions []Mention
	var locations [][]int
	buffer := &strings.Builder{}

	// this loop is the string parser
	// implementing a state machine using mode
	for i, r := range message {
		switch r {
		case '<':
			// not tracking anything, so let's start
			if mode == pmodeInit {
				mode = pmodeOpen
				start = i
				continue
			}

			// not sure what we got, but it's not what we expected
			buffer.Reset()
			mode = pmodeInit

		case '>':
			switch mode {
			case pmodeInit:
				continue

			case pmodeUser: // complete user ID
				if buffer.Len() < 2 {
					break
				}

				mentions = append(mentions, Mention{ID: buffer.String(), Type: TypeUser})
				locations = append(locations, []int{start, i})

			case pmodeGroup: // complete group ID
				if buffer.Len() == 0 {
					break
				}

				mentions = append(mentions, Mention{ID: buffer.String(), Type: TypeGroup})
				locations = append(locations, []int{start, i})

			case pmodeHash:
				if buffer.Len() < 2 {
					break
				}

				mentions = append(mentions, Mention{ID: buffer.String(), Type: TypeChannelRef})
				locations = append(locations, []int{start, i})

			case pmodePipe:
				if len(tmp) < 2 {
					break
				}

				mentions = append(mentions, Mention{ID: tmp, Label: buffer.String(), Type: TypeChannelRef})
				locations = append(locations, []int{start, i})

			case pmodeEx: // @here, @channel, @everyone?
				switch id := buffer.String(); id {
				case "here", "channel", "everyone":
					mentions = append(mentions, Mention{ID: channelID, Type: typeFromStr(id)})
					locations = append(locations, []int{start, i})
				}
			}

			// not pmodeInit, so reset
			tmp = ""
			buffer.Reset()
			mode = pmodeInit

		case '@':
			// if previous character was <
			if mode == pmodeOpen {
				mode = pmodeAt
				continue
			}

			// we should be in init phase
			if mode != pmodeInit {
				buffer.Reset()
				mode = pmodeInit
			}

		case '!':
			// if previous character was <
			if mode == pmodeOpen {
				mode = pmodeEx
				continue
			}

			// we should be in init phase
			if mode != pmodeInit {
				buffer.Reset()
				mode = pmodeInit
			}

		case '#':
			if mode == pmodeOpen {
				mode = pmodeHash
				continue
			}

		case 'U', 'W':
			// if mode is not in ...
			if mode&(pmodeAt|pmodeUser|pmodeGroup|pmodeEx|pmodeHash) == 0 {
				continue
			}

			if mode == pmodeAt {
				mode = pmodeUser
			}

			if buffer.Len() >= 64 { // FAILSAFE: buffer shouldn't be this long ಠ_ಠ
				buffer.Reset()
				mode = pmodeInit
				continue
			}

			buffer.WriteRune(r)

		case '^':
			if mode == pmodeEx {
				if buffer.String() == "subteam" {
					mode = pmodeGroup
					buffer.Reset()
					continue
				}

				buffer.Reset()
				mode = pmodeInit
				continue
			}

			if mode != pmodeInit {
				buffer.Reset()
				mode = pmodeInit
			}

		case '|':
			if mode == pmodeHash {
				// a channel ID should have at least two characters
				// if not this is garbage state: re-init
				if buffer.Len() < 2 {
					buffer.Reset()
					mode = pmodeInit
					continue
				}

				tmp = buffer.String()
				buffer.Reset()
				mode = pmodePipe
				continue
			}

			if mode != pmodeInit {
				buffer.Reset()
				mode = pmodeInit
			}

		default:
			if mode == pmodeInit {
				continue
			}

			// if mode in pmodeAt or pmodeOpen
			if mode&(pmodeAt|pmodeOpen) > 0 {
				mode = pmodeInit
				buffer.Reset()
				continue
			}

			// if mode in pmodeEx, pmodeUser, or pmodeGroup
			if mode&(pmodeEx|pmodeUser|pmodeGroup|pmodeHash|pmodePipe) > 0 {
				if buffer.Len() >= 64 { // FAILSAFE: buffer shouldn't be this long ಠ_ಠ
					buffer.Reset()
					mode = pmodeInit
					continue
				}

				buffer.WriteRune(r)
			}
		}
	}

	return mentions, locations
}
