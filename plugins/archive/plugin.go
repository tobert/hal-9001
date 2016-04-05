package archive

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/netflix/hal-9001/hal"
	"github.com/nlopes/slack"
)

// ArchiveEntry is a single event observed by the archive plugin.
type ArchiveEntry struct {
	Timestamp time.Time `json: timestamp`
	User      string    `json: user`
	Room      string    `json: room`
	Broker    string    `json: broker`
	Body      string    `json: body`
}

// ArchiveTable stores events for posterity.
// The brokers currently supported do not provide a surrogate event id
// and instead rely on the timestamp/user/room for identity.
const ArchiveTable = `
CREATE TABLE IF NOT EXISTS archive (
  ts       TIMESTAMP,
  user     VARCHAR(64),
  room     VARCHAR(255),
  broker   VARCHAR(255),
  body     TEXT,
  PRIMARY KEY (ts,user,room,broker)
)`

const ReactionTable = `
CREATE TABLE IF NOT EXISTS reactions (
  ts       TIMESTAMP,
  user     VARCHAR(64),
  room     VARCHAR(255),
  broker   VARCHAR(255),
  reaction VARCHAR(64),
  PRIMARY KEY (ts,user,room,broker)
)`

func Register() {
	archive := hal.Plugin{
		Name: "message_archive",
		Func: archiveRecorder,
	}
	archive.Register()

	reactions := hal.Plugin{
		Name: "reaction_tracker",
		Func: archiveReactionAdded,
	}
	reactions.Register()

	// apply the schema to the database as necessary
	hal.SqlInit(ArchiveTable)
	hal.SqlInit(ReactionTable)

	http.HandleFunc("/v1/archive", httpGetArchive)
}

// ArchiveRecorder inserts every message received into the database for use
// by other parts of the system.
func archiveRecorder(evt hal.Evt) {
	db := hal.SqlDB()

	sql := `INSERT INTO archive (ts, user, room, broker, body) VALUES (?, ?, ?, ?, ?)`
	_, err := db.Exec(sql, evt.Time, evt.User, evt.Room, evt.BrokerName(), evt.Body)
	if err != nil {
		log.Printf("Could not insert event into archive: %s\n", err)
	}
}

// archiveReactionAdded records a reaction event in the database.
func archiveReactionAdded(evt hal.Evt) {
	switch evt.Original.(type) {
	case slack.ReactionAddedEvent:
		rae := evt.Original.(slack.ReactionAddedEvent)
		log.Printf("Slack Reaction: %v\n", rae)
	default:
		log.Printf("Unknown Reaction: %q\n", evt.Body)
	}
}

// httpGetArchive retreives the 50 latest items from the event archive.
func httpGetArchive(w http.ResponseWriter, r *http.Request) {
	aes, err := FetchArchive(50)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not fetch message archive: '%s'", err), 500)
		return
	}

	js, err := json.Marshal(aes)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not marshal archive to json: '%s'", err), 500)
		return
	}

	w.Write(js)
}

// FetchArchive selects messages from the archive table up to the provided number of messages limit.
func FetchArchive(limit int) ([]*ArchiveEntry, error) {
	db := hal.SqlDB()

	sql := `SELECT ts, user, room, broker, body
	          FROM archive
			  WHERE ts > (NOW() - INTERVAL '1 day')
			  ORDER BY ts DESC`

	rows, err := db.Query(sql)
	if err != nil {
		log.Printf("archive query failed: %s\n", err)
		return nil, err
	}
	defer rows.Close()

	aes := []*ArchiveEntry{}

	for rows.Next() {
		ae := ArchiveEntry{}

		err = rows.Scan(&ae.Timestamp, &ae.User, &ae.Room, &ae.Broker, &ae.Body)
		if err != nil {
			log.Printf("Row iteration failed: %s\n", err)
			return nil, err
		}

		aes = append(aes, &ae)
	}

	return aes, nil
}
