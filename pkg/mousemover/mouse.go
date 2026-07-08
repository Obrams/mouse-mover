package mousemover

import (
	"time"

	"github.com/go-vgo/robotgo"
)

// retryDelay is the pause between attempts inside the reliable move/click loops.
const retryDelay = 20 * time.Millisecond

// mouseController abstracts the OS mouse so the action logic can be unit-tested
// without touching real hardware (the real implementation is robotgoController).
type mouseController interface {
	getPos() (x, y int)
	move(x, y int)
	click(button string) bool
}

// robotgoController is the production implementation backed by robotgo.
type robotgoController struct{}

func (robotgoController) getPos() (int, int) { return robotgo.GetMousePos() }

func (robotgoController) move(x, y int) { robotgo.Move(x, y) }

func (robotgoController) click(button string) bool {
	//robotgo.Click does not return a status; the OS either delivers the synthetic
	//event or silently drops it (e.g. missing Accessibility permission on macOS).
	//verifyClickable is what actually gates whether the pipeline is working.
	robotgo.Click(button, false)
	return true
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

// verifyClickable is the SEPARATE verification step for the click: it proves the
// OS is accepting our synthetic input by nudging the pointer 1px and back and
// confirming the move registered. It always leaves the cursor where it found it.
func verifyClickable(c mouseController) bool {
	startX, startY := c.getPos()
	c.move(startX+1, startY+1)
	movedX, movedY := c.getPos()
	c.move(startX, startY) //restore original position
	return movedX != startX || movedY != startY
}

// stableClick makes the click reliable: on each attempt it first verifies the
// input pipeline (verifyClickable) and only then clicks, retrying the whole
// sequence until it succeeds. It returns true only when a click was issued
// against a pipeline that was verified working, so a "success" is never a lie.
func stableClick(c mouseController, button string, attempts int) bool {
	for i := 0; i < attempts; i++ {
		if !verifyClickable(c) {
			time.Sleep(retryDelay)
			continue
		}
		if c.click(button) {
			return true
		}
		time.Sleep(retryDelay)
	}
	return false
}

// performAction is one idle-heartbeat's worth of work: reliably move the pointer
// (this is what keeps the machine awake) and, when enabled, issue a verified
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
