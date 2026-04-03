//go:build !windows

package tray

import (
	"os"
	"os/signal"
	"syscall"
)

// Actions defines the minimal interface the agent runtime exposes to the tray helper.
type Actions interface {
	OpenToday()
	OpenDashboard()
	Stop()
}

// Run blocks until termination signal is delivered, then stops the agent loop.
func Run(actions Actions) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	actions.Stop()
}
