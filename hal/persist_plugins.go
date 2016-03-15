package hal

import (
	"log"
)

const PLUGIN_INST_TABLE = `
CREATE TABLE IF NOT EXISTS plugin_instances (
	plugin  varchar(255) NOT NULL,
	broker  varchar(255) NOT NULL,
	channel varchar(255) NOT NULL,
	regex   varchar(255) NOT NULL DEFAULT "",
	ts      TIMESTAMP,
	PRIMARY KEY(plugin, broker, channel)
)
`

// LoadInstances loads the previously saved plugin instance configuration
// from the database and *merges* it with the plugin registry. This should be
// idempotent if run multiple times.
// TODO: decide if it makes sense to persist settings or just pull the prefs
// each time.
func (pr *pluginRegistry) LoadInstances() error {
	log.Printf("Loading plugin instances to the database.")
	defer func() { log.Printf("Done loading plugin instances.") }()

	SqlInit(PLUGIN_INST_TABLE)

	db := SqlDB()
	rows, err := db.Query(`SELECT plugin, broker, channel, regex FROM plugin_instances`)
	if err != nil {
		log.Printf("LoadInstances SQL query failed: %s", err)
		return err
	}

	defer rows.Close()

	var pname, bname, channelId, re string
	for rows.Next() {
		err := rows.Scan(&pname, &bname, &channelId, &re)
		if err != nil {
			log.Printf("LoadInstances rows.Scan() failed: %s", err)
			return err
		}

		found := pr.FindInstances(pname, channelId)
		if len(found) == 0 {
			// instance is in the DB but not registered, do it now
			plugin := pr.GetPlugin(pname)

			inst := plugin.Instance(channelId)
			inst.Regex = re // RE can be overridden per instance

			// go over the settings and pull preferences before firing up the instance
			inst.LoadSettingsFromPrefs()

			err = inst.Register()
			if err != nil {
				log.Printf("Could not register plugin instance for plugin %q and channel id %q: %s",
					pname, channelId, err)
				return err
			}
		} else if len(found) == 1 {
			// already there, move on
			continue
		} else {
			// hal doesn't currently support > 1 plugin instance per channel
			log.Fatalf("BUG: more than 1 plugin instance matched for plugin %q and channel id %q",
				pname, channelId)
		}
	}

	return nil
}

// SaveInstances saves plugin instances configurations to the database.
func (pr *pluginRegistry) SaveInstances() error {
	log.Printf("Saving plugin instances to the database.")
	defer func() { log.Printf("Done saving plugin instances.") }()

	SqlInit(PLUGIN_INST_TABLE)

	instances := pr.InstanceList()

	// use a transaction to (relatively) safely wipe & rewrite the whole table
	db := SqlDB()
	tx, err := db.Begin()
	stmt, err := tx.Prepare(`INSERT INTO plugin_instances
	                          (plugin, broker, channel, regex)
	                         VALUES (?, ?, ?, ?)`)

	// clear the table before writing new records
	_, err = tx.Exec("TRUNCATE TABLE plugin_instances")

	for _, inst := range instances {
		_, err = stmt.Exec(inst.Plugin.Name, inst.Broker.Name(), inst.ChannelId, inst.Regex)
		if err != nil {
			log.Printf("insert failed: %s", err)
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("SaveInstances transaction failed: %s", err)
		return err
	}

	return nil
}
