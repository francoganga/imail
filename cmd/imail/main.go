package main

import (
	"fmt"
	"log"
	"os"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/gookit/goutil/dump"
)

func main2() {
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

	seqset := new(imap.SeqSet)
	uid := uint32(826)
	log.Printf("trying to fetch uid %d", uid)
	seqset.AddNum(uid)

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)

	go func() {
		done <- mc.UidFetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()

	msg := <-messages
	s := msg.Envelope.Subject

	fmt.Printf("subject=%v\n", s)

	if err := <-done; err != nil {
		log.Fatal(err)
	}

}

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

	uids := make(chan uint32, 1)

	for up := range updates {
		dump.P(up)
		switch update := up.(type) {
		case *client.MailboxUpdate:
			uids <- update.Mailbox.UidNext
			// we close the channel after the first update
			// TODO: rembember to make a new one for the next idle
			close(updates)
			fmt.Println("closing updates")
		}
	}

	// WARNING:
	// rembember to close the channel if we want to send commands to the server
	close(stop)

	// WARNING: Wait for idle
	// If we dont wait for it to stop we have a data race
	// TODO: main loop should be:
	// for {
	// idle()
	// range updates for one update
	// wait for idle to stop
	// start IDLE again
	// }
	if err := <-doneIdle; err != nil {
		log.Printf("error idling: %v", err)
	}

	fmt.Println("Done idling....")

	for uid := range uids {
		messages := make(chan *imap.Message, 1)
		doneFetch := make(chan error, 1)

		seqset := new(imap.SeqSet)
		seqset.AddNum(uid)

		go func() {
			doneFetch <- mc.UidFetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
		}()

		msg := <-messages
		s := msg.Envelope.Subject

		fmt.Printf("subject=%v\n", s)

		if err := <-doneFetch; err != nil {
			log.Fatal(err)
		}
	}

}

