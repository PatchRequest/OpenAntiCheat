// hydra_ws_server.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	s := NewHydraWSServer("0.0.0.0:8080", "/", 1024)
	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
	log.Println("listening on :8080 / (binary ACEvent)")

	// consumer
	go func() {
		for ev := range s.Recv {
			if js, err := ev.ToJSON(); err == nil {
				fmt.Println(js)
			}
		}
	}()

	// block
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	_ = s.Close()
}
