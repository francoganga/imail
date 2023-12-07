package main

import (
	"fmt"
	"log"
	"os"

	"os/exec"

	"github.com/emersion/go-imap/client"
)

func main() {
	// TODO: this probably could be configurable for now only support gmail
	mc, err := client.DialTLS("imap.gmail.com:993", nil)

	if err != nil {
		panic(err)
	}

	err = mc.Login("fmanuelganga@gmail.com", os.Getenv("PASS"))

	if err != nil {
		panic(err)
	}

	defer mc.Logout()

	if _, err := mc.Select("INBOX", false); err != nil {
		log.Fatal(err)
	}

	// Create a channel to receive mailbox updates
	updates := make(chan client.Update)
	mc.Updates = updates

	stop := make(chan struct{})
	doneIdle := make(chan error, 1)
	go func() {
		doneIdle <- mc.Idle(stop, nil)
	}()

	fmt.Println("Start idling....")
	// Listen for updates

	for up := range updates {
		switch up.(type) {
		case *client.MailboxUpdate:
			go func() {
				cmd := exec.Command("notify-send", "Mail", "You have new mail", "--icon=mail-unread")
				err := cmd.Run()

				if err != nil {
					log.Fatal(err)
				}
			}()
		}
	}
}

