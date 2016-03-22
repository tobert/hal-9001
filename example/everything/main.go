package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/netflix/hal-9001/hal"

	"github.com/netflix/hal-9001/brokers/hipchat"
	"github.com/netflix/hal-9001/brokers/slack"

	"github.com/netflix/hal-9001/plugins/archive"
	"github.com/netflix/hal-9001/plugins/autoresponder"
	"github.com/netflix/hal-9001/plugins/pagerduty"
	"github.com/netflix/hal-9001/plugins/pluginmgr"
	"github.com/netflix/hal-9001/plugins/prefmgr"
	"github.com/netflix/hal-9001/plugins/roster"
	"github.com/netflix/hal-9001/plugins/uptime"
)

func main() {
	// configuration is in environment variables
	// if you prefer configuration files or flags, that's cool, just replace
	// this part with your thing
	dsn := requireEnv("HAL_DSN")
	keyfile := requireEnv("HAL_SECRETS_KEY_FILE")
	controlRoom := requireEnv("HAL_CONTROL_ROOM")
	hipchatRoomJid := requireEnv("HAL_HIPCHAT_ROOM_JID")
	hipchatRoomName := requireEnv("HAL_HIPCHAT_ROOM_NAME")
	webAddr := defaultEnv("HAL_HTTP_LISTEN_ADDR", ":9001")

	// hal provides a k/v API for managing secrets that the DB code uses to get
	// its DSN (which contains a password). Put the DSN there so the DB can find
	// it.
	secrets := hal.Secrets()
	secrets.Set(hal.SECRETS_KEY_DSN, dsn)

	// parts of hal rely on the database (prefs, secrets, etc.)
	// so make sure the DSN is valid and hal can connect before
	// doing anything else
	// hal can't do much without the database, so you probably want this
	db := hal.SqlDB()
	if err := db.Ping(); err != nil {
		log.Fatalf("Could not ping the database: %s", err)
	}

	// get the secrets encryption key from the file specified
	// this should be protected like any other private key
	// if you don't use the secrets persistence, this can be removed/ignored
	skey, err := ioutil.ReadFile(keyfile)
	if err != nil {
		log.Fatalf("Could not read key file '%s': %s", keyfile, err)
	}

	// Set the encryption key for persisted secrets.
	// Secrets can persist to the database, encrypting the key and value
	// with AES-GCM before writing so that database backups, etc only contain
	// ciphertext and no cleartext secrets.
	secrets.SetEncryptionKey(skey)

	// load secrets from the database
	secrets.LoadFromDB()

	// update the DSN again since the database might have a stale copy
	secrets.Set(hal.SECRETS_KEY_DSN, dsn)

	// the generic broker is virtual and built into hal
	// if you only care about one broker you can ignore it but many of the
	// builtin plugins rely on it and will have to be converted (easy though)
	gbroker := hal.GetGenericBroker()

	// plugins are registered at startup but not bound to any events
	// until they are activated/instantiated at runtime
	// These plugins are generic and can be attached to any room / any time.
	autoresponder.Register(gbroker)
	pagerduty.Register(gbroker)
	pluginmgr.Register(gbroker)
	prefmgr.Register(gbroker)
	roster.Register(gbroker)
	uptime.Register(gbroker)

	// load any previously configured plugin instances from the database
	pr := hal.PluginRegistry()
	pr.LoadInstances()

	// pluginmgr is needed to set up all the other plugins
	// so if it's not present, initialize it manually just this once
	// alternatively, you could poke config straight into the DB
	// TODO: remove the hard-coded room name or make it configurable
	if len(pr.FindInstances(controlRoom, "pluginmgr")) == 0 {
		mgr := pr.GetPlugin("pluginmgr")
		mgrInst := mgr.Instance(controlRoom)
		mgrInst.Register()
	}

	// configure the Hipchat broker
	hconf := hipchat.Config{
		Host:     hipchat.HIPCHAT_HOST, // TODO: not really configurable yet
		Jid:      secrets.Get("hipchat.jid"),
		Password: secrets.Get("hipchat.password"),

		// TODO: make this configurable via prefs (or maybe secrets?)
		Rooms: map[string]string{
			hipchatRoomJid: hipchatRoomName,
		},
	}
	hc := hconf.NewBroker("hipchat")

	// configure the Slack broker
	sconf := slack.Config{
		Token: secrets.Get("slack.token"),
	}
	slk := sconf.NewBroker("slack")

	// the archive plugin uses the Slack API to record stars and reactions
	archive.Register(slk)

	// bind the slack and hipchat plugins to the router
	// the generic broker gets copies of all the events these emit
	router := hal.Router()
	router.AddBroker(hc)
	router.AddBroker(slk)

	// start up the router goroutine
	go router.Route()

	// temporary ... (2016-03-02)
	// TODO: remove this or make it permanent by using the same method as
	// the pluginmgr bootstrap above to set the room name, etc.
	slk.Send(hal.Evt{
		Body:   "Ohai! HAL-9001 up and running.",
		Room:   controlRoom,
		User:   "HAL-9001",
		Broker: slk,
	})

	// start the webserver - some plugins register handlers to the default
	// net/http router. This makes them available. Remove this if you don't
	// want the webserver and the handlers will be silently ignored.
	go func() {
		err := http.ListenAndServe(webAddr, nil)
		if err != nil {
			log.Fatalf("Could not listen on '%s': %s\n", webAddr, err)
		}
	}()

	// block forever
	select {}
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("The %q environment variable is required!", key)
	}

	return val
}

func defaultEnv(key, def string) string {
	val := os.Getenv(key)

	if val == "" {
		return def
	}

	return val
}
