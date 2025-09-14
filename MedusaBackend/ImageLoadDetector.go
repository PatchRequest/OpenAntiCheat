// detector.go
package main

import "fmt"

type ImageLoadDetector struct {
	events <-chan Event
}

func NewImageLoadDetector(eventCh <-chan Event) *ImageLoadDetector {
	return &ImageLoadDetector{
		events: eventCh,
	}
}

func (d *ImageLoadDetector) Start() {
	for event := range d.events {

		if event.Type != "load_image" {
			continue
		}
		if event.ProcessID != event.ToProctedPID {
			continue
		}

		fmt.Println("[Image Load]")
		fmt.Println(event)

	}
}
