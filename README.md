# shutdown
Shutdown management library for Go

This package helps you manage shutdown code centrally, and provides functionality to execute code when shutdown occurs.

This will enable you to save data, notify other services that your application is shutting down.

Package home: https://github.com/klauspost/shutdown

[![GoDoc][1]][2] [![Build Status][3]][4]

[1]: https://godoc.org/github.com/klauspost/shutdown?status.svg
[2]: https://godoc.org/github.com/klauspost/shutdown
[3]: https://travis-ci.org/klauspost/shutdown.svg
[4]: https://travis-ci.org/klauspost/shutdown

# concept
Managing shutdowns can be very tricky, often leading to races, crashes and strange behaviour.
This package will help you manage the shutdown process and will attempt to fix some of the common problems when dealing with shutting down.

The shutdown package allow you to block shutdown while certain parts of your code is running. This is helpful to ensure that operations are not interupted.

The second part of the shutdown process is notifying goroutines in a select loop and calling functions in your code that handles various shutdown procedures, like closing databases, notifying other servers, deleting temporary files, etc.

The second part of the process has three **stages**, which will enable you to do your shutdown in stages. This will enable you to rely on some parts, like logging, to work in the first two stages. There is no rules for what you should put in which stage, but things executing in stage one can safely rely on stage two not being executed yet.

All operations have **timeouts**. This is to fix another big issue with shutdowns; applications that hang on shutdown. The timeout is for each stage of the shutdown process, and can be adjusted to your application needs. If a timeout is exceeded the next shutdown stage will be initiated regardless.

Finally, you can always cancel a notifier, which will remove it from the shutdown queue.

# usage

First get the libary with `go get -u github.com/klauspost/shutdown`, and add it as an import to your code with `import github.com/klauspost/shutdown`.

The next thing you probably want to do is to register Ctrl+c and system terminate. This will make all shutdown handlers run when any of these are sent to your program:
```Go
	shutdown.OnSignal(0, os.Interrupt, syscall.SIGTERM)
```

If you don't like the default timeout duration of 5 seconds, you can change it by calling the `SetTimeout` function:
```Go
  shutdown.SetTimeout(time.Second * 1)
```
Now the maximum delay for shutdown is **4 seconds**. The timeout is applied to each of the stages and that is also the maximum time to wait for the shutdown to begin.

Next you can register functions to run when shutdown runs:
```Go
  logFile := os.Create("log.txt")
  
  // Execute the function in the first stage of the shutdown process
  _ = shutdown.FirstFunc(func(interface{}){
    logFile.Close()
  }, nil)
  
  // Execute this function in the second part of the shutdown process
  _ = shutdown.SecondFunc(func(interface{}){
    _ = os.Delete("log.txt")
  }, nil)
```
As noted there are three stages. All functions in one stage are executed in parallel, but the package will wait for all functions in one stage to have finished before moving on to the next one.  So your code cannot rely on any particular order of execution inside a single stage, but you are guaranteed that the First stage is finished before any functions from stage two are executed.

This example above uses functions that are called, but you can also request channels that are notified on shutdown. This allows you do have shutdown handling in blocked select statements like this:

```Go
  go func() {
    // Get a stage 1 notification
    finish := shutdown.First()
    select {
      case n:= <-finish:
        log.Println("Closing")
        close(n)
        return
  }
```
It is important that you close the channel you receive. This is your way of signalling that you are done. If you do not close the channel you get shutdown will wait until the timeout has expired before proceeding to the next stage.

If you for some reason don't need a notifier anymore you can cancel it. When a notifier has been cancelled it will no longer receive notifications, and the shutdown code will no longer wait for it on exit.
```Go
  go func() {
    // Get a stage 1 notification
    finish := shutdown.First()
    select {
      case n:= <-finish:
        close(n)
        return
      case <-otherchan:
        finish.Cancel()
        return
  }
```
Functions are cancelled the same way by cancelling the returned notifier. Be aware that if shutdown has been initiated you can no longer cancel notifiers, so you may need to aquire a shutdown lock (see below).

The final thing you can do is to lock shutdown in parts of your code you do not want to be interrupted by a shutdown, or if the code relies on resources that are destroyed as part of the shutdown process.

A simple example can be seen in this http handler:
```Go
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
	  // Acquire a lock. While this is held server will not shut down (except after timeout)
		if shutdown.Lock() {
			defer shutdown.Unlock()
			io.WriteString(w, "Server running")
		} else {
		  // Shutdown has started, return that the service is unavailable
		  w.WriteHeader(http.StatusServiceUnavailable)
		  w.Write([]byte("Server is now shutting down"))
		  return
		}
	})
```
If shutdown is started, either by a signal or by another goroutine, it will wait until the lock is released. It is important always to release the lock, if shutdown.Lock() returns true. Otherwise the server will have to wait until the timeout has passed before it starts shutting down, which may not be what you want.

Finally you can call `shutdown.Exit(exitcode)` to call all exit handlers and exit your application. This will wait for all locks to be released and notify all shutdown handlers and exit with the given exit code. If you want to do the exit yourself you can call the `shutdown.Shutdown()`, whihc does the same, but doesn't exit. Beware that you don't hold a lock when you call Exit/Shutdown.


Also there are some things to be mindful of:
* Notifiers **can** be created inside shutdown code, but only for stages **following** the current. So stage 1 notifiers can create stage 2 notifiers, but if they create a stage 1 notifier this will never be called.
* Timeout cannot be changed once shutdown has been initiated. It will remain what it was when shutdown was started.
* Notifiers returned from a function (eg. FirstFunc) can be used for selects. They will be notified, but the shutdown manager will not wait for them to finish, so using them for this is not recommended.

When you design with this do take care that this library is for **controlled** shutdown of your application. If you application crashes no shutdown handlers are run, so panics will still be fatal. You can of course still call the `Shutdown()` function if you recover a panic, but the library does nothing like this automatically.

# examples

There are examples in the [examples folder](https://github.com/klauspost/shutdown/tree/master/examples).

# license

This code is published under an MIT license. See LICENSE file for more information.
