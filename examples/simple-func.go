// +build ignore

package main

import (
	"github.com/klauspost/shutdown"
	"log"
	"net/http"
	"os"
	"sync"
	"syscall"
)

// This example shows a server that has logging to a file
//
// When the webserver is closed, it will close the file when all requests have
// been finished.
//
// In a real world, you would not want multiple goroutines writing to the same file
//
// To execute, use 'go run simple-func.go'

var logFile *os.File

func main() {
	// Make shutdown catch Ctrl+c and system terminate
	shutdown.OnSignal(0, os.Interrupt, syscall.SIGTERM)

	// Create a log file
	logFile, _ = os.Create("log.txt")

	// When shutdown is initiated, close the file
	shutdown.FirstFunc(func(interface{}) {
		log.Println("Closing log...")
		logFile.Close()
	}, nil)

	// Start a webserver
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		// Get a lock, and write to the file if we get it.
		// While we have the lock the file will not be closed.
		if shutdown.Lock() {
			_, _ = logFile.WriteString(req.URL.String() + "\n")
			shutdown.Unlock()
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
