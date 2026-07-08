package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/getlantern/systray"
	"github.com/go-vgo/robotgo"
	"github.com/kirsle/configdir"
	"github.com/Obrams/mouse-mover/assets/icon"
	"github.com/Obrams/mouse-mover/pkg/mousemover"
	log "github.com/sirupsen/logrus"
)

// AppSettings is persisted to settings.json. The zero value for the tuning
// fields means "use the app default" (see mousemover.Config.normalize).
type AppSettings struct {
	Icon         string `json:"icon"`
	IdleSeconds  int    `json:"idleSeconds,omitempty"`
	MovePixels   int    `json:"movePixels,omitempty"`
	ClickEnabled bool   `json:"clickEnabled,omitempty"`
	ClickButton  string `json:"clickButton,omitempty"`
}

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "1.3.0"

var configPath = configdir.LocalConfig("mm")
var configFile = filepath.Join(configPath, "settings.json")

func main() {
	systray.Run(onReady, onExit)
}

func applyIcon(iconName string) {
	switch iconName {
	case "cloud":
		systray.SetIcon(icon.CloudIcon)
	case "man":
		systray.SetIcon(icon.ManIcon)
	case "geometric":
		systray.SetIcon(icon.GeometricIcon)
	default:
		systray.SetIcon(icon.Data)
	}
}

// saveSettings writes the whole settings struct, so changing one field (e.g. the
// icon from the menu) never wipes the tuning fields the user configured by hand.
func saveSettings(settings AppSettings) {
	fh, err := os.Create(configFile)
	if err != nil {
		log.Errorf("could not write settings file %v: %v", configFile, err)
		return
	}
	defer fh.Close()
	if err := json.NewEncoder(fh).Encode(settings); err != nil {
		log.Errorf("could not encode settings: %v", err)
	}
}

// loadSettings reads settings.json, creating it with defaults if missing. It
// always returns usable settings, falling back to defaults on any error instead
// of crashing the tray app.
func loadSettings() AppSettings {
	settings := AppSettings{Icon: "mouse"}

	if err := configdir.MakePath(configPath); err != nil {
		log.Errorf("could not create config dir %v, using defaults: %v", configPath, err)
		return settings
	}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		saveSettings(settings)
		return settings
	}

	fh, err := os.Open(configFile)
	if err != nil {
		log.Errorf("could not open config file %v, using defaults: %v", configFile, err)
		return settings
	}
	defer fh.Close()
	if err := json.NewDecoder(fh).Decode(&settings); err != nil {
		log.Errorf("could not read settings, using defaults: %v", err)
		return AppSettings{Icon: "mouse"}
	}
	return settings
}

func configFromSettings(settings AppSettings) mousemover.Config {
	return mousemover.Config{
		IdleSeconds:  settings.IdleSeconds,
		MovePixels:   settings.MovePixels,
		ClickEnabled: settings.ClickEnabled,
		ClickButton:  settings.ClickButton,
	}
}

func onReady() {
	go func() {
		settings := loadSettings()
		applyIcon(settings.Icon)
		cfg := configFromSettings(settings)

		about := systray.AddMenuItem("About MM", "Information about the app")
		systray.AddSeparator()
		mmStart := systray.AddMenuItem("Start", "start the app")
		mmStop := systray.AddMenuItem("Stop", "stop the app")

		icons := systray.AddMenuItem("Icons", "icon of the app")
		mouse := icons.AddSubMenuItem("Mouse", "Mouse icon")
		mouse.SetIcon(icon.Data)
		cloud := icons.AddSubMenuItem("Cloud", "Cloud icon")
		cloud.SetIcon(icon.CloudIcon)
		man := icons.AddSubMenuItem("Man", "Man icon")
		man.SetIcon(icon.ManIcon)
		geometric := icons.AddSubMenuItem("Geometric", "Geometric")
		geometric.SetIcon(icon.GeometricIcon)

		mmStop.Disable()
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

		mouseMover := mousemover.GetInstance()
		mouseMover.Start(cfg)
		mmStart.Disable()
		mmStop.Enable()

		chooseIcon := func(name string) {
			settings.Icon = name
			applyIcon(name)
			saveSettings(settings)
		}

		for {
			select {
			case <-mmStart.ClickedCh:
				log.Infof("starting the app")
				mouseMover.Start(cfg)
				mmStart.Disable()
				mmStop.Enable()

			case <-mmStop.ClickedCh:
				log.Infof("stopping the app")
				mmStart.Enable()
				mmStop.Disable()
				mouseMover.Quit()

			case <-mQuit.ClickedCh:
				log.Infof("Requesting quit")
				mouseMover.Quit()
				systray.Quit()
				return
			case <-mouse.ClickedCh:
				chooseIcon("mouse")
			case <-cloud.ClickedCh:
				chooseIcon("cloud")
			case <-man.ClickedCh:
				chooseIcon("man")
			case <-geometric.ClickedCh:
				chooseIcon("geometric")
			case <-about.ClickedCh:
				log.Infof("Requesting about")
				robotgo.Alert("Mouse-mover (MM) app v"+version, "Originally developed by Prashant Gupta. \n\nMore info at: https://github.com/Obrams/mouse-mover", "OK", "")
			}
		}
	}()
}

func onExit() {
	// clean up here
	log.Infof("Finished quitting")
}
