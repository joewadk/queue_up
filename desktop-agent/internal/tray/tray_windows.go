//go:build windows
package tray

import _ "embed"

import (
	"log"

	"github.com/getlantern/systray"
)

//go:embed assets/queue_up.ico
var trayIcon []byte

type Actions interface {
	OpenToday()
	MarkDone()
	Stop()
}
//run the system tray with the provided actions for each menu item. this will block until the user quits the tray.
func Run(actions Actions) {
	onReady := func() {
		systray.SetIcon(trayIcon) 
		systray.SetTitle("Queue Up")
		systray.SetTooltip("Queue Up Desktop Agent")

		openItem := systray.AddMenuItem("Open Today's Problem", "Open current recommended LeetCode problem")
		doneItem := systray.AddMenuItem("Mark as Done", "Mark today's problem complete")
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("Quit", "Stop the desktop agent")

		go func() {
			for {
				select {
				case <-openItem.ClickedCh:
					actions.OpenToday()
				case <-doneItem.ClickedCh:
					actions.MarkDone()
				case <-quitItem.ClickedCh:
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
