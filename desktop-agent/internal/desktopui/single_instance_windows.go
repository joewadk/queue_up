//go:build windows && !no_gl

package desktopui

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

const dashboardMutexName = "Global\\QueueUpDesktopDashboardSingleton"

func acquireDashboardInstanceLock() (func(), error) {
	namePtr, err := syscall.UTF16PtrFromString(dashboardMutexName)
	if err != nil {
		return nil, fmt.Errorf("encode dashboard mutex name: %w", err)
	}

	handle, createErr := windows.CreateMutex(nil, false, namePtr)
	if createErr != nil {
		return nil, fmt.Errorf("create dashboard mutex: %w", createErr)
	}

	lastErr := windows.GetLastError()
	if lastErr == syscall.ERROR_ALREADY_EXISTS {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("dashboard is already open")
	}

	release := func() {
		_ = windows.CloseHandle(handle)
	}
	return release, nil
}
