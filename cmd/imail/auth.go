package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
)

var auth_url_raw = "https://accounts.google.com/o/oauth2/v2/auth"

func auth() {

	redirect_uri, ok := os.LookupEnv("REDIRECT_URI")
	if !ok {
		log.Print("REDIRECT_URI not set")
		os.Exit(1)
	}
	client_id, ok := os.LookupEnv("CLIENT_ID")

	if !ok {
		log.Print("CLIENT_ID not set")
		os.Exit(1)
	}

	authUrl, err := url.Parse(auth_url_raw)

	if err != nil {
		panic(err)
	}

	query := url.Values{}

	query.Add("scope", "https://mail.google.com/")
	// Response type code/token
	query.Add("response_type", "code")

	// State is optional
	// TODO: add this for better security
	// query.Add("state", "security_token=138r5719ru3e1&url=https://oauth2.example.com/token")
	query.Add("redirect_uri", redirect_uri)
	query.Add("client_id", client_id)

	authUrl.RawQuery = query.Encode()

	cmd := exec.Command("xdg-open", authUrl.String())

	err = cmd.Start()

	if err != nil {
		panic(err)
	}

	fmt.Println("Continue authentication on browser...")
}

