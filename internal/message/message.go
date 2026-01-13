package message

import "time"

type Platform int

const (
	Twitch Platform = iota
	YouTube
)

func (p Platform) String() string {
	switch p {
	case Twitch:
		return "TW"
	case YouTube:
		return "YT"
	default:
		return "??"
	}
}

type Message struct {
	Platform  Platform
	Username  string
	Timestamp time.Time
	Content   string
}
