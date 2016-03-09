package pagerduty

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// AuthenticatedGet authenticates with the provided token and GETs the url
// with the query sent in the body as "query=%s", query.
func authenticatedGet(url, token string, query string) (*http.Response, error) {
	tokenHdr := fmt.Sprintf("Token token=%s", token)

	buf := bytes.NewBuffer([]byte{})
	if query != "" {
		fmt.Fprintf(buf, "query=%s", query)
	}

	req, err := http.NewRequest("GET", url, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", tokenHdr)

	client := &http.Client{}
	return client.Do(req)
}

// AuthenticatedPost authenticates with the provided token and posts the
// provided body.
func authenticatedPost(token string, body []byte) (*http.Response, error) {
	tokenHdr := fmt.Sprintf("Token token=%s", token)
	buf := bytes.NewBuffer(body)

	// TODO: make Endpoint a url parameter
	req, err := http.NewRequest("POST", Endpoint, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", tokenHdr)

	client := &http.Client{}
	return client.Do(req)
}

func pagedUrl(path, domain string, offset, limit int) string {
	url := fmt.Sprintf("https://%s.pagerduty.com%s", domain, path)

	query := make([]string, 0)

	if limit != 0 {
		query = append(query, fmt.Sprintf("limit=%d", limit))
	}

	if offset != 0 {
		query = append(query, fmt.Sprintf("offset=%d", offset))
	}

	if len(query) > 0 {
		log.Printf("pagedUrl: %s?%s", url, strings.Join(query, "&"))
		return fmt.Sprintf("%s?%s", url, strings.Join(query, "&"))
	}

	log.Printf("pagedUrl(%q, %q, %d, %d): %s", path, domain, offset, limit, url)
	return url
}
