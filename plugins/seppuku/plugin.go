package seppuku

/*
 * Copyright 2016 Albert P. Tobey <atobey@netflix.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import (
	"log"
	"os"
	"time"

	"github.com/netflix/hal-9001/hal"
)

func Register() {
	p := hal.Plugin{
		Name:  "seppuku",
		Func:  seppuku,
		Regex: "^!seppuku",
	}
	p.Register()
}

func seppuku(evt hal.Evt) {
	evt.Reply("sayonara")
	time.Sleep(2 * time.Second)
	log.Printf("exiting due to !sayonara command from %s in %s/%s", evt.User, evt.BrokerName(), evt.Room)
	os.Exit(1337)
}
