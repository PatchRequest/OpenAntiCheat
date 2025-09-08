// detector.go
package main

import "fmt"

type DetectorModule struct {
	events <-chan Event
}

func NewDetectorModule(eventCh <-chan Event) *DetectorModule {
	return &DetectorModule{
		events: eventCh,
	}
}

func (d *DetectorModule) Start(handle func(Event)) {
	for event := range d.events {
		fmt.Println(event)
	}
}
