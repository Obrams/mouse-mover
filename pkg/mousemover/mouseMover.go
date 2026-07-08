package mousemover

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/prashantgupta24/activity-tracker/pkg/activity"
	"github.com/prashantgupta24/activity-tracker/pkg/tracker"
)

var (
	instance      *MouseMover
	instanceMutex sync.Mutex
)

const (
	//actionTimeout guards against robotgo hanging. It must comfortably exceed the
	//worst-case retry budget of a move+click sequence.
	actionTimeout = 2 * time.Second
	logDir        = "log"
	logFileName   = "logFile-mm-5"
)

// Start the main app with the given configuration. Zero/invalid config fields
// fall back to sensible defaults (see Config.normalize).
func (m *MouseMover) Start(cfg Config) {
	if m.state.isRunning() {
		return
	}
	m.state = &state{}
	m.quit = make(chan struct{})
	m.config = cfg.normalize()
	if m.controller == nil {
		m.controller = robotgoController{}
	}

	workerInterval := 10

	activityTracker := &tracker.Instance{
		HeartbeatInterval: m.config.IdleSeconds, //value always in seconds
		WorkerInterval:    workerInterval,
		// LogLevel:          "debug", //if we want verbose logging
	}

	heartbeatCh := activityTracker.Start()
	m.run(heartbeatCh, activityTracker)
}

func (m *MouseMover) run(heartbeatCh chan *tracker.Heartbeat, activityTracker *tracker.Instance) {
	go func() {
		state := m.state
		if state != nil && state.isRunning() {
			return
		}
		//capture the quit channel this run was started with, so a later
		//Start()/Quit() that swaps m.quit cannot leave this goroutine orphaned.
		quit := m.quit
		state.updateRunningStatus(true)

		logger := getLogger(m, false, logFileName) //set writeToFile=true only for debugging
		movePixel := m.config.MovePixels
		for {
			select {
			case heartbeat := <-heartbeatCh:
				if !heartbeat.WasAnyActivity {
					if state.isSystemSleeping() {
						logger.Infof("system sleeping")
						continue
					}
					//buffered so performAction never blocks on send if we time out below
					mouseMoveSuccessCh := make(chan bool, 1)
					go func(pixel int) {
						mouseMoveSuccessCh <- m.performAction(pixel)
					}(movePixel)
					select {
					case wasMouseMoveSuccess := <-mouseMoveSuccessCh:
						if wasMouseMoveSuccess {
							state.updateLastMouseMovedTime(time.Now())
							logger.Infof("Is system sleeping? : %v : moved mouse at : %v\n\n", state.isSystemSleeping(), state.getLastMouseMovedTime())
							movePixel *= -1
							state.updateDidNotMoveCount(0)
						} else {
							didNotMoveCount := state.getDidNotMoveCount()
							state.updateDidNotMoveCount(didNotMoveCount + 1)
							lastErrorTime := state.getLastErrorTime()
							msg := fmt.Sprintf("Mouse pointer cannot be moved at %v. Last moved at %v. Happened %v times. (Only notifies once every 24 hours.) See README for details.",
								time.Now(), state.getLastMouseMovedTime(), state.getDidNotMoveCount())
							logger.Error(msg)
							//show only 1 error in a 24 hour window. lastErrorTime holds the time
							//of the last notification, so compare against it *before* updating it.
							if state.getDidNotMoveCount() >= 10 && (lastErrorTime.IsZero() || time.Since(lastErrorTime).Hours() > 24) {
								state.updateLastErrorTime(time.Now())
								go func() {
									robotgo.Alert("Error with Automatic Mouse Mover", msg)
								}()
							}
						}
					case <-time.After(actionTimeout):
						//timeout, do nothing
						logger.Errorf("timeout happened after %v while trying to move mouse", actionTimeout)
					}
				} else {
					logger.Infof("activity detected in the last %v seconds.", int(activityTracker.HeartbeatInterval))
					logger.Infof("Activity type:\n")
					for activityType, times := range heartbeat.ActivityMap {
						logger.Infof("activityType : %v times: %v\n", activityType, len(times))
						if activityType == activity.MachineSleep {
							state.updateMachineSleepStatus(true)
							logger.Infof("system sleep registered. Is system sleeping? : %v", state.isSystemSleeping())
							break
						} else {
							state.updateMachineSleepStatus(false)
						}
					}
					logger.Infof("\n\n\n")
				}
			case <-quit:
				logger.Infof("stopping mouse mover")
				state.updateRunningStatus(false)
				activityTracker.Quit()
				return
			}
		}
	}()
}

// Quit the app
func (m *MouseMover) Quit() {
	//stopIfRunning atomically clears the running flag and reports whether we were
	//running, so concurrent/duplicate Quit calls close the channel at most once.
	if m != nil && m.state.stopIfRunning() {
		close(m.quit)
	}
	if m.logFile != nil {
		m.logFile.Close()
	}
}

// GetInstance gets the singleton instance for mouse mover app
func GetInstance() *MouseMover {
	instanceMutex.Lock()
	defer instanceMutex.Unlock()
	if instance == nil {
		instance = &MouseMover{
			state:      &state{},
			config:     DefaultConfig(),
			controller: robotgoController{},
		}
	}
	return instance
}
