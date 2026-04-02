//go:build windows

package tray

import (
	"log"

	"github.com/getlantern/systray"

	"queue_up/desktop-agent/internal/appicon"
)

type Actions interface {
	OpenToday()
	OpenDashboard()
	Stop()
}

// run the system tray with the provided actions for each menu item. this will block until the user quits the tray.
func Run(actions Actions) {
	onReady := func() {
		systray.SetIcon(appicon.Bytes())
		systray.SetTitle("Queue Up")
		systray.SetTooltip("Queue Up Desktop Agent")

		openItem := systray.AddMenuItem("Open Today's Problem", "Open current recommended LeetCode problem")
		dashboardItem := systray.AddMenuItem("Open Dashboard", "Open Queue Up Desktop dashboard")
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("Quit", "Stop the desktop agent")

		go func() {
			for {
				select {
				case <-openItem.ClickedCh:
					actions.OpenToday()
				case <-dashboardItem.ClickedCh:
					actions.OpenDashboard()
				case <-quitItem.ClickedCh:
					terminateQueueUpAgents()
					actions.Stop()
					systray.Quit()
					return
				}
			}
		}()
	}

	onExit := func() {
		log.Printf("tray exited")
	}

	systray.Run(onReady, onExit)
}
