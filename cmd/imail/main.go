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
	done := make(chan error, 1)
	go func() {
		done <- mc.Idle(stop, nil)
	}()

	fmt.Println("Start idling....")
	// Listen for updates

	for {
		select {
		case update := <-updates:
			log.Println("New update:", update)
			dump.P(update)

			switch u := update.(type) {
			case *client.MailboxUpdate:
				log.Println("is Mailbox update")

				seqset := new(imap.SeqSet)
				uid := u.Mailbox.UidNext
				log.Printf("trying to fetch uid %d", uid)
				seqset.AddNum(uid)

				messages := make(chan *imap.Message, 1)
				done := make(chan error, 1)

				go func() {
					done <- mc.UidFetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
				}()

				msg := <-messages

				subject := msg.Envelope.Subject
				log.Printf("Subject: %s", subject)

				if err := <-done; err != nil {
					log.Fatal(err)
				}

			default:
				log.Println("Unknown update")
			}
		case err := <-done:
			if err != nil {
				log.Fatal(err)
			}
			log.Println("Not idling anymore")
			return
		}
	}

}

