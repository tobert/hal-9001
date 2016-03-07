# Hal-9001

A bot library written in Go.

Hal is a Go library that offers a number of facilities for creating a bot
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

* Go 1.5 with GOVENDOREXPERIMENT=1 or Go >= 1.6

It should build with older versions of Go as long as you have all of the
dependencies in your GOPATH.

# Building

```
go get github.com/nlopes/slack
go get github.com/codegangsta/cli
go get github.com/mattn/go-xmpp
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
	"fmt"
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
// The GenericBroker receives messages from all brokers wired up to
// hal. Plugins have to be associated with a broker at startup
// so Register accepts a hal.Broker (interface) object that it
// can use to create the plugin struct that is then registered with
// the bot. If your plugin uses, e.g. Slack or Hipchat specific functionality,
// use the specific broker and you won't have to deal with weird things
// happening when messages come from an unsupported broker.
func Register(gb *hal.GenericBroker) {
	p := hal.Plugin{
		Name:   "uptime",
		Func:   uptime,
		Regex:  "^!uptime",
		Broker: gb,
	}
	p.Register()
}

// uptime implements the plugin itself
func uptime(evt hal.Evt) {
	ut := time.Since(booted)
	evt.Replyf("uptime: %s", ut.String())
}
```

# TODO

* work on the TODOs sprinkled throughout the code

# FUTURE IDEAS

* [in progress] a Docker plugin that runs code in Docker over stdio
    * exists, but is not ready to be released yet
* integrate sshchat as a broker or an maybe an ssh server for admin stuff

# AUTHOR

Al Tobey <atobey@netflix.com>

# LICENSE

Apache 2
