package events

func ShouldNotify(event Event) bool {
	switch event {
	case Started, Succeeded, Failed, Interrupted, Stopped:
		return true
	default:
		return false
	}
}
