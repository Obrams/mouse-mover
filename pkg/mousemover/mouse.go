package mousemover

import (
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
)

const (
	// retryDelay is the pause between attempts inside the reliable move/click loops.
	retryDelay = 20 * time.Millisecond
	// hookInitDelay gives the OS event tap time to register before we post a click.
	// gohook polls its C event queue every ~50ms, so this must be comfortably above that.
	hookInitDelay = 100 * time.Millisecond
	// clickConfirmTimeout bounds how long we wait for the OS to report our click back.
	clickConfirmTimeout = 500 * time.Millisecond
)

// hookMutex serialises use of the gohook event tap. gohook keeps global state
// (one channel, one C event loop), so only one confirmation may run at a time.
var hookMutex sync.Mutex

// mouseController abstracts the OS mouse so the action logic can be unit-tested
// without touching real hardware (the real implementation is robotgoController).
type mouseController interface {
	getPos() (x, y int)
	move(x, y int)
	// clickConfirmed posts a click and returns true ONLY if the OS event stream
	// reported the click back to us, i.e. the click really happened at the OS
	// level. A false result means the click was not observed and should be retried.
	clickConfirmed(button string) bool
}

// robotgoController is the production implementation backed by robotgo + gohook.
type robotgoController struct{}

func (robotgoController) getPos() (int, int) { return robotgo.GetMousePos() }

func (robotgoController) move(x, y int) { robotgo.Move(x, y) }

// clickConfirmed installs a global mouse event tap, synthesises the click, and
// confirms success only when the tap observes the resulting click event within
// clickConfirmTimeout. This is real proof the click entered the OS event stream
// (not just that we called an API that may have been silently dropped).
func (robotgoController) clickConfirmed(button string) bool {
	hookMutex.Lock()
	defer hookMutex.Unlock()

	evChan := hook.Start()
	defer hook.End()

	// Let the event tap come up before we post the click, otherwise we might
	// post-and-miss the event we are waiting for.
	time.Sleep(hookInitDelay)

	robotgo.Click(button, false)

	deadline := time.After(clickConfirmTimeout)
	for {
		select {
		case ev, ok := <-evChan:
			if !ok {
				return false
			}
			// Any mouse button transition confirms a click reached the OS.
			if ev.Kind == hook.MouseDown || ev.Kind == hook.MouseUp || ev.Kind == hook.MouseHold {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// moveMouse performs a single move by movePixel and reports whether the pointer
// actually moved. A false result means the OS rejected our synthetic event
// (on macOS: the Accessibility permission has not been granted).
func moveMouse(c mouseController, movePixel int) bool {
	startX, startY := c.getPos()
	c.move(startX+movePixel, startY+movePixel)
	gotX, gotY := c.getPos()
	return gotX != startX || gotY != startY
}

// moveMouseReliably retries moveMouse until it registers or attempts run out,
// so a transient rejection doesn't cause a spurious "could not move" error.
func moveMouseReliably(c mouseController, movePixel, attempts int) bool {
	for i := 0; i < attempts; i++ {
		if moveMouse(c, movePixel) {
			return true
		}
		time.Sleep(retryDelay)
	}
	return false
}

// verifyClickable is a fast pre-check that the OS is accepting our synthetic
// input at all: it nudges the pointer 1px and back and confirms the move
// registered. It always leaves the cursor where it found it.
func verifyClickable(c mouseController) bool {
	startX, startY := c.getPos()
	c.move(startX+1, startY+1)
	movedX, movedY := c.getPos()
	c.move(startX, startY) //restore original position
	return movedX != startX || movedY != startY
}

// stableClick makes the click reliable AND confirmed: on each attempt it first
// checks the input pipeline (verifyClickable), then posts the click and waits
// for the OS to report it back (clickConfirmed). It retries until confirmed and
// returns true only when the click was actually observed by the OS event stream,
// so a "success" is never a lie.
func stableClick(c mouseController, button string, attempts int) bool {
	for i := 0; i < attempts; i++ {
		if !verifyClickable(c) {
			time.Sleep(retryDelay)
			continue
		}
		if c.clickConfirmed(button) {
			return true
		}
		time.Sleep(retryDelay)
	}
	return false
}

// performAction is one idle-heartbeat's worth of work: reliably move the pointer
// (this is what keeps the machine awake) and, when enabled, issue a confirmed
// stable click. It returns true only if every required step succeeded.
func (m *MouseMover) performAction(movePixel int) bool {
	if !moveMouseReliably(m.controller, movePixel, m.config.MoveAttempts) {
		return false
	}
	if m.config.ClickEnabled {
		return stableClick(m.controller, m.config.ClickButton, m.config.ClickAttempts)
	}
	return true
}
