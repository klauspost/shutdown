// Copyright (c) 2015 Klaus Post, released under MIT License. See LICENSE file.

// Package shutdown provides management of your shutdown process.
//
// The package will enable you to get notifications for your application and handle the shutdown process.
//
// See more information about the how to use it in the README.md file
//
// Package home: https://github.com/klauspost/shutdown
package shutdown

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"time"
)

// Notifier is a channel, that will be sent a channel
// once the application shuts down.
// When you have performed your shutdown actions close the channel you are given.
type Notifier chan chan struct{}

type fnNotify struct {
	client   Notifier
	internal Notifier
	cancel   chan struct{}
}

var sqM sync.Mutex // Mutex for below
var shutdownQueue [3][]Notifier
var shutdownFnQueue [3][]fnNotify

var srM sync.RWMutex // Mutex for below
var shutdownRequested = false
var timeout = 5 * time.Second

// The maximum delay to wait for each stage to finish.
// When the timeout has expired for a stage the next stage will be initiated.
func SetTimeout(d time.Duration) {
	srM.Lock()
	timeout = d
	srM.Unlock()
}

// Cancel a Notifier.
// This will remove a notifier from the shutdown queue,
// and it will not be signalled when shutdown starts.
// If the shutdown has already started this will not have any effect.
func (s *Notifier) Cancel() {
	srM.RLock()
	if shutdownRequested {
		srM.RUnlock()
		return
	}
	srM.RUnlock()
	sqM.Lock()
	var a chan chan struct{}
	var b chan chan struct{}
	a = *s
	for n := 0; n < 3; n++ {
		for i := range shutdownQueue[n] {
			b = shutdownQueue[n][i]
			if a == b {
				shutdownQueue[n] = append(shutdownQueue[n][:i], shutdownQueue[n][i+1:]...)
			}
		}
		for i, fn := range shutdownFnQueue[n] {
			b = fn.client
			if a == b {
				// Find the matching internal and remove that.
				for i := range shutdownQueue[n] {
					b = shutdownQueue[n][i]
					if fn.internal == b {
						shutdownQueue[n] = append(shutdownQueue[n][:i], shutdownQueue[n][i+1:]...)
					}
				}
				// Cancel, so the goroutine exits.
				close(fn.cancel)
				// Remove this
				shutdownFnQueue[n] = append(shutdownFnQueue[n][:i], shutdownFnQueue[n][i+1:]...)
			}
		}
	}
	sqM.Unlock()
}

// First returns a notifier that will be called in the first stage of shutdowns
func First() Notifier {
	return onShutdown(0)
}

type ShutdownFn func(interface{})

// FirstFunc executes a function in the first stage of the shutdown
func FirstFunc(fn ShutdownFn, v interface{}) Notifier {
	return onFunc(0, fn, v)
}

// Second returns a notifier that will be called in the second stage of shutdowns
func Second() Notifier {
	return onShutdown(1)
}

// SecondFunc executes a function in the second stage of the shutdown
func SecondFunc(fn ShutdownFn, v interface{}) Notifier {
	return onFunc(1, fn, v)
}

// Third returns a notifier that will be called in the third stage of shutdowns
func Third() Notifier {
	return onShutdown(2)
}

// ThirdFunc executes a function in the third stage of the shutdown
// The returned Notifier is only really useful for cancelling the shutdown function
func ThirdFunc(fn ShutdownFn, v interface{}) Notifier {
	return onFunc(2, fn, v)
}

// Create a function notifier.
func onFunc(prio int, fn ShutdownFn, i interface{}) Notifier {
	f := fnNotify{
		internal: onShutdown(prio),
		cancel:   make(chan struct{}),
		client:   make(Notifier, 1),
	}
	go func() {
		select {
		case <-f.cancel:
			return
		case c := <-f.internal:
			{
				defer func() {
					if r := recover(); r != nil {
						log.Println("Panic in shutdown function:", r)
					}
					if c != nil {
						close(c)
					}
				}()
				fn(i)
			}
		}
	}()
	sqM.Lock()
	shutdownFnQueue[prio] = append(shutdownFnQueue[prio], f)
	sqM.Unlock()
	return f.client
}

// onShutdown will request a shutdown notifier.
func onShutdown(prio int) Notifier {
	srM.RLock()
	// If shutdown has already been requested,
	// return a notifier that has already been triggered.
	if shutdownRequested {
		srM.RUnlock()
		n := make(Notifier, 1)
		n <- make(chan struct{})
		return n
	}
	srM.RUnlock()
	sqM.Lock()
	n := make(Notifier, 1)
	shutdownQueue[prio] = append(shutdownQueue[prio], n)
	sqM.Unlock()
	return n
}

// OnSignal will start the shutdown when any of the given signals arrive
//
// A good shutdown default is
//    shutdown.OnSignal(0, os.Interrupt, syscall.SIGTERM)
// which will do shutdown on Ctrl+C and when the program is terminated.
func OnSignal(exitCode int, sig ...os.Signal) {
	// capture signal and shut down.
	c := make(chan os.Signal, 1)
	signal.Notify(c, sig...)
	go func() {
		for _ = range c {
			Shutdown()
			os.Exit(exitCode)
		}
	}()
}

// Exit performs shutdown operations and exits with the given exit code.
func Exit(code int) {
	Shutdown()
	os.Exit(code)
}

// Shutdown will signal all notifiers in three stages.
func Shutdown() {
	srM.Lock()
	shutdownRequested = true
	to := timeout
	srM.Unlock()
	sqM.Lock()
	defer sqM.Unlock()
	for stage, queue := range shutdownQueue {
		n := len(queue)
		if n == 0 {
			continue
		}
		log.Println("Shutdown stage", stage+1)
		wait := make([]chan struct{}, n)

		// Send notification to all waiting
		for i := range queue {
			wait[i] = make(chan struct{})
			queue[i] <- wait[i]
		}

		// Send notification to all function notifiers, but don't wait
		for _, notifier := range shutdownFnQueue[stage] {
			notifier.client <- make(chan struct{})
			close(notifier.client)
		}

		// Wait for all to return, no more than the shutdown delay
		timeout := time.After(to)
		for i := range wait {
			select {
			case <-wait[i]:
			case <-timeout:
				log.Println("timeout waiting to shutdown, forcing shutdown")
				break
			}
		}
	}
	// Reset - mainly for tests.
	shutdownQueue = [3][]Notifier{}
	shutdownFnQueue = [3][]fnNotify{}
}

// Started returns true if shutdown has been started.
// Note that shutdown can have been started before you check the value.
func Started() bool {
	srM.RLock()
	started := shutdownRequested
	srM.RUnlock()
	return started
}
