package roster

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/netflix/hal-9001/hal"
)

type RosterUser struct {
	Broker    string    `json: broker` // broker name e.g. slack, hipchat
	Username  string    `json: username`
	Timestamp time.Time `json: timestamp`
	Channel   string    `json: channel`
}

const ROSTER_TABLE = `
CREATE TABLE IF NOT EXISTS roster (
	broker   VARCHAR(64) NOT NULL,
	username VARCHAR(64) NOT NULL,
	channel  VARCHAR(255) DEFAULT NULL,
	ts       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (broker, username, channel)
)`

func Register(gb *hal.GenericBroker) {
	// rostertracker gets all messages and keeps a database of when users
	// were last seen to support !last, and the web roster.
	roster := hal.Plugin{
		Name:   "roster_tracker",
		Func:   rostertracker,
		Regex:  "",
		Broker: gb,
	}
	roster.Register()

	// TODO: the reply is still slack-specific, fix that! (maybe?)
	rostercmd := hal.Plugin{
		Name:   "roster_command",
		Func:   rosterlast,
		Regex:  "!last",
		Broker: gb,
	}
	rostercmd.Register()

	hal.SqlInit(ROSTER_TABLE)

	http.HandleFunc("/v1/roster", webroster)
}

// rostertracker is called for every message. It grabs the user and current
// time and throws it into the db for later use.
func rostertracker(msg hal.Evt) {
	db := hal.SqlDB()

	sql := `INSERT INTO roster
						(broker, username, channel, ts)
			VALUES (?,?,?,?)
			ON DUPLICATE KEY
			UPDATE broker=?, username=?, channel=?, ts=?`

	params := []interface{}{
		msg.Broker.Name(), msg.From, msg.Channel, msg.Time,
		msg.Broker.Name(), msg.From, msg.Channel, msg.Time,
	}

	_, err := db.Exec(sql, params...)
	if err != nil {
		log.Printf("roster_tracker write failed: %s", err)
	}
}

// rosterlast is the response to !last that causes the bot to reply via DM
// to the user with a table of when users last posted a message to slack
// rather than relying on status, which is usually useless.
func rosterlast(msg hal.Evt) {
	log.Printf("rosterlast(%q)", msg.Body)

	rus, err := GetRoster()
	if err != nil {
		log.Printf("Error while retreiving roster: %s\n", err)
		return
	}

	log.Printf("rosterlast(%v): ", rus)

	// TODO: ASCII art instead of JSON
	js, err := json.Marshal(rus)
	if err != nil {
		log.Printf("JSON marshaling failed: %s\n", err)
		return
	}

	msg.Reply(string(js))
}

func webroster(w http.ResponseWriter, r *http.Request) {
	rus, err := GetRoster()
	if err != nil {
		http.Error(w, fmt.Sprintf("could not fetch roster: '%s'", err), 500)
		return
	}

	js, err := json.Marshal(rus)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not marshal roster to json: '%s'", err), 500)
		return
	}

	w.Write(js)
}

func GetRoster() ([]*RosterUser, error) {
	db := hal.SqlDB()

	sql := `SELECT broker, username, channel, ts FROM roster ORDER BY ts DESC`
	rows, err := db.Query(sql)
	if err != nil {
		log.Printf("Roster query failed: %s\n", err)
		return nil, err
	}
	defer rows.Close()

	rus := []*RosterUser{}

	for rows.Next() {
		ru := RosterUser{}

		err = rows.Scan(&ru.Broker, &ru.Username, &ru.Channel, &ru.Timestamp)
		if err != nil {
			log.Printf("Row iteration failed: %s\n", err)
			return nil, err
		}

		rus = append(rus, &ru)
	}

	return rus, nil
}
