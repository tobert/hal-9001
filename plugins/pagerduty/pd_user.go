package pagerduty

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"time"
)

// API docs: https://developer.pagerduty.com/documentation/rest/users/on_call

type OnCallResponse struct {
	Users              []User `json:"users"`
	ActiveAccountUsers int    `json:"active_account_users"`
	Limit              int    `json:"limit"`
	Query              string `json:"query"`
	Offset             int    `json:"offset"`
	Total              int    `json:"total"`
}

type User struct {
	Id              string   `json:"id"`
	Name            string   `json:"name"`
	Email           string   `json:"email"`
	JobTitle        string   `json:"job_title"`
	Timezone        string   `json:"time_zone"`
	Color           string   `json:"color"`
	Role            string   `json:"role,omitempty"`
	AvatarUrl       string   `json:"avatar_url,omitempty"`
	Billed          bool     `json:"billed,omitempty"`
	UserUrl         string   `json:"user_url,omitempty"`
	InvitationSent  bool     `json:"invitation_sent,omitempty"`
	MarketingOptOut bool     `json:"marketing_opt_out,omitempty"`
	OnCall          []OnCall `json:"on_call,omitempty"`
}

type OnCall struct {
	Level            int              `json:"level"`
	Start            *time.Time       `json:"start"`
	End              *time.Time       `json:"end"`
	EscalationPolicy EscalationPolicy `json:"escalation_policy,omitempty"`
	User             User             `json:"user,omitempty"`
}

func GetUsersOnCall(token, domain string) ([]User, error) {
	users := make([]User, 0)
	oresp := OnCallResponse{}
	offset := 0
	limit := 100

	for {
		url := pagedUrl("/api/v1/users/on_call", domain, offset, limit)

		resp, err := authenticatedGet(url, token, "")
		if err != nil {
			log.Printf("GET %s failed: %s", url, err)
			return users, err
		}

		data, err := ioutil.ReadAll(resp.Body)

		err = json.Unmarshal(data, &oresp)
		if err != nil {
			log.Printf("json.Unmarshal failed: %s", err)
			return []User{}, err
		}

		users = append(users, oresp.Users...)

		if oresp.Total > oresp.Offset {
			offset = oresp.Offset
		} else {
			break
		}
	}

	return users, nil
}
