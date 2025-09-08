// detector.go
package main

import "fmt"

type HandleDetector struct {
	events <-chan Event
	seen   map[string]struct{} // key: path|hash
}

func NewHandleDetector(eventCh <-chan Event) *HandleDetector {
	return &HandleDetector{
		events: eventCh,
	}
}

func (d *HandleDetector) Start() {
	if d.seen == nil {
		d.seen = make(map[string]struct{}, 1024)
	}
	for event := range d.events {

		if event.ProcessID != event.ToProctedPID {
			continue
		}

		if event.Type != "ob_operation" {
			continue
		}

		if event.Path == "" || event.PathHash == "" {
			continue
		}
		key := event.Path + "|" + event.PathHash
		if _, ok := d.seen[key]; ok {
			continue // already seen
		}
		d.seen[key] = struct{}{}

		// first-time sighting — emit a message
		// adapt to your logging or channel if needed
		fmt.Printf("[FIRST_SEEN] path=%q hash=%s pid=%d age=%d lifetime=%d op=%d caller=%d ts=%d\n",
			event.Path, event.PathHash, event.ProcessID, event.PathAge, event.Lifetime,
			event.Operation, event.CallerPID, event.Timestamp)

	}
}
