//go:build !windows

package tray

//stub iomplementation of the tray for non windows platforms

type Actions interface {
	OpenToday()
	OpenDashboard()
	Stop()
}

func Run(actions Actions) {
	actions.Stop()
}
