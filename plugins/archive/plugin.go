package archive

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	slackBroker "github.com/netflix/hal-9001/brokers/slack"
	"github.com/netflix/hal-9001/hal"
	"github.com/nlopes/slack"
)

type ArchiveEntry struct {
	Timestamp time.Time `json: timestamp`
	Username  string    `json: username`
	Channel   string    `json: channel`
	Text      string    `json: text`
}

const ARCHIVE_TABLE = `
CREATE TABLE IF NOT EXISTS archive (
  ts       TIMESTAMP,
  username VARCHAR(64),
  channel  VARCHAR(255),
  txt      TEXT,
  PRIMARY KEY (ts,username,channel)
)`

func Register(sb *slackBroker.Broker) {
	archive := hal.Plugin{
		Name:   "message_archive",
		Func:   archiveRecorder,
		Broker: sb,
	}
	archive.Register()

	stars := hal.Plugin{
		Name:   "slack_star_tracker",
		Func:   slackArchiveStarAdded,
		Broker: sb,
	}
	stars.Register()

	// apply the schema to the database as necessary
	hal.SqlInit(ARCHIVE_TABLE)

	http.HandleFunc("/v1/archive", httpGetArchive)
}

// ArchiveRecorder inserts every message received into the database for use
// by other parts of the system.
func archiveRecorder(msg hal.Evt) {
	db := hal.SqlDB()

	insert := `INSERT INTO archive (ts, username, channel, txt) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(insert, msg.Time, msg.From, msg.Channel, msg.Body)
	if err != nil {
		log.Printf("Could not insert user into roster: %s\n", err)
	}
}

// slackArchiveStarAdded records a star added event in the database.
func slackArchiveStarAdded(evt hal.Evt) {
	sa := evt.Original.(slack.StarAddedEvent)
	log.Printf("Star Added: %v\n", sa)
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

	fetch := `
SELECT ts, username, channel, txt
FROM archive
WHERE ts > (NOW() - INTERVAL '1 day')
ORDER BY ts DESC`
	rows, err := db.Query(fetch)
	if err != nil {
		log.Printf("archive query failed: %s\n", err)
		return nil, err
	}
	defer rows.Close()

	aes := []*ArchiveEntry{}

	for rows.Next() {
		ae := ArchiveEntry{}

		err = rows.Scan(&ae.Timestamp, &ae.Username, &ae.Channel, &ae.Text)
		if err != nil {
			log.Printf("Row iteration failed: %s\n", err)
			return nil, err
		}

		aes = append(aes, &ae)
	}

	return aes, nil
}
