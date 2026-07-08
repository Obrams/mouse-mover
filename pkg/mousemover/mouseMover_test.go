package mousemover

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/prashantgupta24/activity-tracker/pkg/activity"
	"github.com/prashantgupta24/activity-tracker/pkg/tracker"
)

// fakeController is a test double for mouseController. It is safe for concurrent
// use because performAction runs in its own goroutine while the test asserts.
type fakeController struct {
	mu      sync.Mutex
	x, y    int
	canMove bool //when false, move() is a no-op, simulating a blocked input pipeline
	confirm bool //value returned by clickConfirmed (did the OS "observe" the click)
	clicks  int
}

func (f *fakeController) getPos() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.x, f.y
}

func (f *fakeController) move(x, y int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.canMove {
		f.x, f.y = x, y
	}
}

func (f *fakeController) clickConfirmed(button string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clicks++
	return f.confirm
}

func (f *fakeController) clickCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.clicks
}

type TestMover struct {
	suite.Suite
	activityTracker *tracker.Instance
	heartbeatCh     chan *tracker.Heartbeat
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(TestMover))
}

// Run once before all tests
func (suite *TestMover) SetupSuite() {
	heartbeatInterval := 60
	workerInterval := 10

	suite.activityTracker = &tracker.Instance{
		HeartbeatInterval: heartbeatInterval,
		WorkerInterval:    workerInterval,
	}

	suite.heartbeatCh = make(chan *tracker.Heartbeat)
}

// Run once before each test
func (suite *TestMover) SetupTest() {
	instance = nil
}

func (suite *TestMover) TestAppStart() {
	t := suite.T()
	mouseMover := GetInstance()
	mouseMover.run(suite.heartbeatCh, suite.activityTracker)
	time.Sleep(time.Millisecond * 500) //wait for app to start
	assert.True(t, mouseMover.state.isRunning(), "app should have started")
}

func (suite *TestMover) TestSingleton() {
	t := suite.T()

	mouseMover1 := GetInstance()
	mouseMover1.run(suite.heartbeatCh, suite.activityTracker)

	time.Sleep(time.Millisecond * 500)

	mouseMover2 := GetInstance()
	assert.True(t, mouseMover2.state.isRunning(), "instance should have started")
}

func (suite *TestMover) TestLogFile() {
	t := suite.T()
	mouseMover := GetInstance()
	logFileName := "test1"

	getLogger(mouseMover, true, logFileName)

	filePath := logDir + "/" + logFileName
	assert.FileExists(t, filePath, "log file should exist")
	os.RemoveAll(filePath)
}

func (suite *TestMover) TestSystemSleepAndWake() {
	t := suite.T()
	mouseMover := GetInstance()

	mouseMover.state = &state{}
	mouseMover.config = DefaultConfig()
	mouseMover.controller = &fakeController{canMove: true}
	state := mouseMover.state
	heartbeatCh := make(chan *tracker.Heartbeat)

	mouseMover.run(heartbeatCh, suite.activityTracker)
	time.Sleep(time.Millisecond * 500) //wait for app to start
	assert.True(t, mouseMover.state.isRunning(), "instance should have started")
	assert.False(t, mouseMover.state.isSystemSleeping(), "machine should not be sleeping")

	//fake a machine-sleep activity
	machineSleepActivityMap := make(map[activity.Type][]time.Time)
	var sleepTimeArray []time.Time
	sleepTimeArray = append(sleepTimeArray, time.Now())
	machineSleepActivityMap[activity.MachineSleep] = sleepTimeArray
	heartbeatCh <- &tracker.Heartbeat{
		WasAnyActivity: true,
		ActivityMap:    machineSleepActivityMap,
	}
	time.Sleep(time.Millisecond * 500) //wait for it to be registered
	assert.True(t, mouseMover.state.isSystemSleeping(), "machine should be sleeping now")

	//assert app is sleeping
	heartbeatCh <- &tracker.Heartbeat{
		WasAnyActivity: false,
	}

	time.Sleep(time.Millisecond * 500) //wait for it to be registered
	assert.True(t, time.Time.IsZero(state.getLastMouseMovedTime()), "should be default but is ", state.getLastMouseMovedTime())
	assert.Equal(t, state.getDidNotMoveCount(), 0, "should be 0")

	//fake a machine-wake activity
	machineWakeActivityMap := make(map[activity.Type][]time.Time)
	var wakeTimeArray []time.Time
	wakeTimeArray = append(wakeTimeArray, time.Now())
	machineWakeActivityMap[activity.MachineWake] = wakeTimeArray
	heartbeatCh <- &tracker.Heartbeat{
		WasAnyActivity: true,
		ActivityMap:    machineWakeActivityMap,
	}

	time.Sleep(time.Millisecond * 500) //wait for it to be registered
	assert.False(t, mouseMover.state.isSystemSleeping(), "machine should be awake now")
}

func (suite *TestMover) TestMouseMoveSuccess() {
	t := suite.T()
	mouseMover := GetInstance()

	mouseMover.state = &state{}
	mouseMover.config = DefaultConfig()
	mouseMover.controller = &fakeController{canMove: true}
	state := mouseMover.state
	heartbeatCh := make(chan *tracker.Heartbeat)

	mouseMover.run(heartbeatCh, suite.activityTracker)
	time.Sleep(time.Millisecond * 500) //wait for app to start
	assert.True(t, state.isRunning(), "instance should have started")
	assert.False(t, state.isSystemSleeping(), "machine should not be sleeping")
	assert.True(t, time.Time.IsZero(state.getLastMouseMovedTime()), "should be default")
	assert.Equal(t, state.getDidNotMoveCount(), 0, "should be 0")

	heartbeatCh <- &tracker.Heartbeat{
		WasAnyActivity: false,
	}

	time.Sleep(time.Millisecond * 500) //wait for it to be registered
	assert.False(t, time.Time.IsZero(state.getLastMouseMovedTime()), "should be default but is ", state.getLastMouseMovedTime())
}

func (suite *TestMover) TestMouseMoveFailure() {
	t := suite.T()
	mouseMover := GetInstance()

	mouseMover.state = &state{}
	mouseMover.config = DefaultConfig()
	mouseMover.controller = &fakeController{canMove: false}
	state := mouseMover.state
	heartbeatCh := make(chan *tracker.Heartbeat)

	mouseMover.run(heartbeatCh, suite.activityTracker)
	time.Sleep(time.Millisecond * 500) //wait for app to start
	assert.True(t, state.isRunning(), "instance should have started")
	assert.False(t, state.isSystemSleeping(), "machine should not be sleeping")
	assert.True(t, time.Time.IsZero(state.getLastMouseMovedTime()), "should be default")
	assert.Equal(t, state.getDidNotMoveCount(), 0, "should be 0")
	assert.True(t, state.getLastErrorTime().IsZero(), "should be default")

	heartbeatCh <- &tracker.Heartbeat{
		WasAnyActivity: false,
	}

	time.Sleep(time.Millisecond * 500) //wait for it to be registered
	assert.True(t, time.Time.IsZero(state.getLastMouseMovedTime()), "should be default but is ", state.getLastMouseMovedTime())
	assert.NotEqual(t, state.getDidNotMoveCount(), 0, "should not be 0")
}

// TestStableClickSucceeds verifies the click is issued exactly once when the
// input pipeline works and the OS confirms the click.
func (suite *TestMover) TestStableClickSucceeds() {
	t := suite.T()
	c := &fakeController{canMove: true, confirm: true}

	ok := stableClick(c, "left", 3)

	assert.True(t, ok, "click should succeed when it is confirmed by the OS")
	assert.Equal(t, 1, c.clickCount(), "exactly one click should be issued")
}

// TestStableClickFailsWhenInputBlocked verifies that when the OS is not
// accepting synthetic input, the pre-check fails and NO click is issued
// (so we never report a click that could not have landed).
func (suite *TestMover) TestStableClickFailsWhenInputBlocked() {
	t := suite.T()
	c := &fakeController{canMove: false}

	ok := stableClick(c, "left", 3)

	assert.False(t, ok, "click should fail when input is not accepted")
	assert.Equal(t, 0, c.clickCount(), "no click should be issued if the pre-check fails")
}

// TestStableClickFailsWhenNotConfirmed verifies that when the click is posted
// but the OS never reports it back, stableClick retries every attempt and then
// fails - it never claims success for an unconfirmed click.
func (suite *TestMover) TestStableClickFailsWhenNotConfirmed() {
	t := suite.T()
	c := &fakeController{canMove: true, confirm: false}

	ok := stableClick(c, "left", 3)

	assert.False(t, ok, "click must fail when the OS does not confirm it")
	assert.Equal(t, 3, c.clickCount(), "every attempt should have tried to click")
}

// TestPerformActionWithClick verifies the full move+click path when clicking is
// enabled: the pointer moves and exactly one confirmed click is issued.
func (suite *TestMover) TestPerformActionWithClick() {
	t := suite.T()
	c := &fakeController{canMove: true, confirm: true}
	mouseMover := &MouseMover{
		state:      &state{},
		controller: c,
		config: Config{
			MovePixels:    10,
			ClickEnabled:  true,
			ClickButton:   "left",
			MoveAttempts:  3,
			ClickAttempts: 3,
		},
	}

	ok := mouseMover.performAction(10)

	assert.True(t, ok, "move+click should succeed")
	assert.Equal(t, 1, c.clickCount(), "exactly one click should be issued")
}

// TestPerformActionMoveOnly verifies that with clicking disabled a successful
// move is enough and no click is issued.
func (suite *TestMover) TestPerformActionMoveOnly() {
	t := suite.T()
	c := &fakeController{canMove: true}
	mouseMover := &MouseMover{
		state:      &state{},
		controller: c,
		config:     DefaultConfig(),
	}

	ok := mouseMover.performAction(10)

	assert.True(t, ok, "move-only action should succeed")
	assert.Equal(t, 0, c.clickCount(), "no click should be issued when disabled")
}
