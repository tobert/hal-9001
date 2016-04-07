package main

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
	"github.com/netflix/hal-9001/brokers/generic"
	"github.com/netflix/hal-9001/hal"
)

// This bot doesn't do anything except set up the generic broker and then
// block forever. The generic broker doesn't produce anything so nothing
// will happen and this is totally useless except to demonstrate the minimum
// amount of hal's API required to start the system.
//
// Most of hal's functionality is optional. It's still built along with the
// rest of hal but is not active unless it's used in main or a plugin.

func main() {
	conf := generic.Config{}
	broker := conf.NewBroker("generic")

	router := hal.Router()
	router.AddBroker(broker)
	router.Route()

	// TODO: maybe add a timer loop to inject some messages and exercise
	// the system a little.
}
