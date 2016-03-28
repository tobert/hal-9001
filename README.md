# Hal-9001

Hal-9001 is a Go library that offers a number of facilities for creating a bot
and its plugins.

# Goals

* make easy things easy and hard things possible
    * check out the uptime plugin for easy
* provide full access to low-level APIs to plugins
    * e.g. Slack plugins can get access to raw events and the full API
    * support multiple event sources with full API pass-through
* provide infrastructure for incident management plugins
    * e.g. reaction tracking, archiving, alternate approaches to presence

# Requirements

* Go >= 1.5

It should build with older versions of Go but it has not been tested.

# Building

A few dependencies are required by Hal's core library and plugins.

```
go get github.com/nlopes/slack
go get github.com/mattn/go-xmpp
go get github.com/codegangsta/cli
go get github.com/go-sql-driver/mysql
```

# Plugins

Hal plugins should be in a package. You can have more than one plugin
per package. Some ship with Hal, others are in their own repos and
can be added with go get/import.

Because plugins are not activated automatically and can be bound to channels
with separate configs, they have to be registered and then instantiated.

```go
package uptime

// uptime: the simplest useful plugin possible

import (
	"time"

	"github.com/netflix/hal-9001/hal"
)

var booted time.Time

func init() {
	booted = time.Now()
}

// The plugin's Register() should be called from main() in the bot to
// make the plugin available for use at runtime. It can be called anything
// you like, but most of the plugins call it Register().
//
// Plugins are not tied to a specific broker so if it is going to use
// the evt.Original field, be careful about double-checking the type
// of message or evt.Broker to make sure it's safe to use.
func Register() {
	p := hal.Plugin{
		Name:   "uptime",
		Func:   uptime,
		Regex:  "^!uptime",
	}

	p.Register()
}

// uptime implements the plugin itself
func uptime(evt hal.Evt) {
	ut := time.Since(booted)
	evt.Replyf("uptime: %s", ut.String())
}
```

# Rationale

Some constructs in Hal are the result of a few decisions that deserve explanation.

## MySQL as the only supported database driver

Right now, only mysql-compatible database backends are supported. This is
unlikely to change. Coding directly against a specific database allows Hal
to use database-specific features and avoid unncessarry abstractions or loss
of power required to support other databases.

Netflix runs its bot in AWS using Aurora with local testing against MariaDB.

## missing tests & ubiquitous assertions

This is not a permanent situation. The API changed a lot as the bot was being
built and tests were frequently invalidated. Now that the API is more stable,
tests are being added back over time.

In order to speed up development and reduce the frequence of error checking
in plugin/bot code, many parts of hal simply crash the program when errors
occur. This makes assumptions about errors obvious and immediately visible
without having to bubble errors up into consumer code at the cost of having
to run your hal bot under a supervisor. When reasons are found to convert
fatal errors into error returns, code should be refactored to do so.

# TODO

* work on the TODOs sprinkled throughout the code
* provide more examples, e.g. slack-only, hipchat-only, console + slack
* logging hooks to redirect logs to a channel
* revive/update the Docker plugin

# FUTURE IDEAS

* [in progress] a Docker plugin that runs code in Docker over stdio
    * exists, but is not ready to be released yet
* integrate sshchat as a broker or an maybe an ssh server for admin stuff

# AUTHOR

Al Tobey <atobey@netflix.com>

# LICENSE

Apache 2
