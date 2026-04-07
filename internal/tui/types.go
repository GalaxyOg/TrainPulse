package tui

import (
	"time"

	"github.com/trainpulse/trainpulse/internal/store"
)

type StopFunc func(runID string) (string, error)
type DoctorFunc func() (string, error)

type Options struct {
	Store           *store.Store
	Stop            StopFunc
	Doctor          DoctorFunc
	Version         string
	StorePath       string
	ConfigPath      string
	ErrorLogPath    string
	RefreshInterval time.Duration
}

type focusArea int

const (
	focusList focusArea = iota
	focusFilter
)

type modalType int

const (
	modalNone modalType = iota
	modalConfirmStop
	modalSearch
	modalSetup
	modalInfo
	modalLogs
	modalCleanup
)

type tickMsg time.Time

type refreshMsg struct {
	runs       []store.Run
	counts     map[string]int
	lastFailed string
	lastActive string
	err        error
}

type actionMsg struct {
	kind    string
	message string
	err     error
}

type logMsg struct {
	runID   string
	path    string
	tail    int
	lines   []string
	summary string
	err     error
}

type setupField struct {
	key   string
	label string
	hint  string
	value string
}

type setupState struct {
	fields []setupField
	index  int
}
