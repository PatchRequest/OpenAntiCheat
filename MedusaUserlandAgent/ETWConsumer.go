package main

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

func ensureETWEnabled(channel string) error {
	const base = `SOFTWARE\Microsoft\Windows\CurrentVersion\WINEVT\Channels\`
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, base+channel, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("open channel key failed: %w", err)
	}
	defer k.Close()

	// already enabled
	if v, _, err := k.GetIntegerValue("Enabled"); err == nil && v == 1 {
		return nil
	}
	if err := k.SetDWordValue("Enabled", 1); err != nil {
		return fmt.Errorf("set Enabled=1 failed: %w", err)
	}
	return nil
}
