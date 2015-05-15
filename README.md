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


# examples

There are examples in the [examples folder](https://github.com/klauspost/shutdown/tree/master/examples).

# license

This code is published under an MIT license. See LICENSE file for more information.
