package pagerduty

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

// API docs: https://developer.pagerduty.com/documentation/rest/escalation_policies/on_call

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
)

func GetServiceByKey(token, serviceKey string) (Service, error) {
	svcs, err := getServices(token, serviceKey)
	if err != nil {
		return Service{}, err
	} else if len(svcs) == 1 {
		return svcs[0], nil
	} else {
		return Service{}, fmt.Errorf("Got %d services, expected 1.", len(svcs))
	}
}

func GetServices(token, serviceKey string) ([]Service, error) {
	return getServices(token, "")
}

func getServices(token, serviceKey string) ([]Service, error) {
	services := make([]Service, 0)
	offset := 0
	limit := 100

	qdata := make(map[string]string)
	if serviceKey != "" {
		qdata["query"] = serviceKey
	}

	for {
		svcResp := ServicesResponse{}

		svcsUrl := pagedUrl("/api/v1/services", offset, limit, qdata)

		resp, err := authenticatedGet(svcsUrl, token)
		if err != nil {
			log.Printf("GET %s failed: %s", svcsUrl, err)
			return services, err
		}

		data, err := ioutil.ReadAll(resp.Body)

		fmt.Printf("\n\n\n%s\n\n\n", data)

		err = json.Unmarshal(data, &svcResp)
		if err != nil {
			log.Printf("json.Unmarshal failed: %s", err)
			return []Service{}, err
		}

		services = append(services, svcResp.Services...)

		if svcResp.Total > svcResp.Offset {
			offset = offset + limit
		} else {
			break
		}
	}

	return services, nil
}

func GetService(token, id string) (Service, error) {
	out := Service{
		IncidentCounts: IncidentCounts{},
	}

	svcsUrl := pagedUrl("/api/v1/services/"+id, 0, 0, nil)

	resp, err := authenticatedGet(svcsUrl, token)
	if err != nil {
		log.Printf("GET %s failed: %s", svcsUrl, err)
		return out, err
	}

	log.Printf("%q", resp.Status)

	data, err := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(data, &out)
	if err != nil {
		log.Printf("json.Unmarshal failed: %s", err)
		return out, err
	}

	return out, nil
}
