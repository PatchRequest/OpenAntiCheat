package main

import "fmt"

func RemoteThreadDetectorLoop(DetectRemotThreadChannel chan interface{}) {
	for event := range DetectRemotThreadChannel {
		switch e := event.(type) {
		case *CreateThreadNotifyRoutineEvent:
			if checkCreatorPIDAndThreadPID(e) {
				// Additional actions can be taken here if needed
			}
		default:
			// Ignore other events
		}
	}
}

func checkCreatorPIDAndThreadPID(event *CreateThreadNotifyRoutineEvent) bool {
	creatorPID := event.CallerPID
	threadPID := event.ProcessID
	threadID := event.ThreadID
	if creatorPID != threadPID {
		fmt.Printf("[INFO] Remote thread detected! CreatorPID: %d, ThreadPID: %d, ThreadID: %d\n", creatorPID, threadPID, threadID)
		return true
	}
	return false
}
