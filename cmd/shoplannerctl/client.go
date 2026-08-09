package main

import (
	"context"
	"net/http"
)

type Client struct {
	http             http.Client
	addr, user, pass string
}

func NewClient(addr string, username, password string) *Client {
	var httpClient http.Client

	return &Client{
		addr: addr,
		http: httpClient,
		user: username,
		pass: password,
	}
}

func (c *Client) auth(ctx context.Context) {
}
