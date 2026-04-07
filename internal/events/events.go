package events

type Event string

const (
	Started     Event = "STARTED"
	Succeeded   Event = "SUCCEEDED"
	Failed      Event = "FAILED"
	Interrupted Event = "INTERRUPTED"
	Stopped     Event = "STOPPED"
	Heartbeat   Event = "HEARTBEAT"
)

func IsTerminalStatus(status string) bool {
	switch status {
	case string(Succeeded), string(Failed), string(Interrupted), string(Stopped):
		return true
	default:
		return false
	}
}
