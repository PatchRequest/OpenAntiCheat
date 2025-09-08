// detector.go
package main

import "fmt"

type RemoteThreadDetector struct {
	events <-chan Event
}

func NewRemoteThreadDetector(eventCh <-chan Event) *RemoteThreadDetector {
	return &RemoteThreadDetector{
		events: eventCh,
	}
}

func (d *RemoteThreadDetector) Start() {
	for event := range d.events {
		if event.ProcessID != event.ToProctedPID {
			continue
		}
		if event.Type != "create_thread" {
			continue
		}
		if event.CallerPID == event.ProcessID {
			continue
		}
		fmt.Printf("[REMOTE_THREAD] pid=%d caller=%d ts=%d lifetime=%d path=%q hash=%s\n",
			event.ProcessID,
			event.CallerPID,
			event.Timestamp,
			event.Lifetime,
			event.Path,
			event.PathHash,
		)

	}
}
