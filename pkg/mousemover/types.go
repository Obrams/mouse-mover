package mousemover

import (
	"os"
	"sync"
	"time"
)

// MouseMover is the main struct for the app
type MouseMover struct {
	quit       chan struct{}
	logFile    *os.File
	state      *state
	config     Config
	controller mouseController
}

// state manages the internal working of the app
type state struct {
	mutex              sync.RWMutex
	isAppRunning       bool
	isSysSleeping      bool
	lastMouseMovedTime time.Time
	lastErrorTime      time.Time
	didNotMoveCount    int
}

// Config controls runtime behaviour. It is safe to build with zero values:
// normalize() replaces any zero/invalid field with a sensible default.
type Config struct {
	//IdleSeconds is how long the machine must be idle before the mouse is nudged.
	IdleSeconds int
	//MovePixels is the distance (in pixels) the pointer is moved each time.
	MovePixels int
	//ClickEnabled, when true, issues a verified mouse click after each move.
	//Off by default because a synthetic click acts on whatever is under the
	//cursor - enable it only if a plain move isn't enough to keep you "active".
	ClickEnabled bool
	//ClickButton is the button used when ClickEnabled is true ("left"/"right"/"center").
	ClickButton string
	//MoveAttempts / ClickAttempts bound the retry loops that make the action reliable.
	MoveAttempts  int
	ClickAttempts int
}

// DefaultConfig returns the built-in defaults.
func DefaultConfig() Config {
	return Config{
		IdleSeconds:   60,
		MovePixels:    10,
		ClickEnabled:  false,
		ClickButton:   "left",
		MoveAttempts:  3,
		ClickAttempts: 3,
	}
}

// normalize fills any unset/invalid field with its default.
func (c Config) normalize() Config {
	d := DefaultConfig()
	if c.IdleSeconds <= 0 {
		c.IdleSeconds = d.IdleSeconds
	}
	if c.MovePixels == 0 {
		c.MovePixels = d.MovePixels
	}
	if c.ClickButton == "" {
		c.ClickButton = d.ClickButton
	}
	if c.MoveAttempts <= 0 {
		c.MoveAttempts = d.MoveAttempts
	}
	if c.ClickAttempts <= 0 {
		c.ClickAttempts = d.ClickAttempts
	}
	return c
}
