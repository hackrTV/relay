package message

import "time"

type Platform int

const (
	Twitch Platform = iota
	YouTube
	HackrTV
)

func (p Platform) String() string {
	switch p {
	case Twitch:
		return "TTV"
	case YouTube:
		return "YT_"
	case HackrTV:
		return "HTV"
	default:
		return "???"
	}
}

type Message struct {
	Platform  Platform
	Username  string
	Timestamp time.Time
	Content   string
}
