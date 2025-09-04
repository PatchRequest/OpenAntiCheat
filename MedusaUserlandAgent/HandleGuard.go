package main

import "fmt"

func HandleGuardLoop(HandleGuardChannel chan interface{}) {
	for event := range HandleGuardChannel {
		switch e := event.(type) {
		case OB_OPERATION_HANDLE_Event:
			if e.ProcessID == ToProtectPID {
				fmt.Printf("[INFO] Handle operation detected for protected PID %d! Operation: %d\n", ToProtectPID, e.Operation)
			}
		default:
			// Ignore other events

		}
	}
}
