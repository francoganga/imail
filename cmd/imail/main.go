package main

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

var lastUid uint32

func main() {
	// TODO: this probably could be configurable for now only support gmail
	mc, err := client.Dial("localhost:143")

	if err != nil {
		panic(err)
	}

	err = mc.Login("ubuntu", "asdfqwer")

	if err != nil {
		panic(err)
	}

	defer mc.Logout()

	if _, err := mc.Select("INBOX", false); err != nil {
		log.Fatal(fmt.Errorf("select INBOX error: %v", err))
	}

	// Create a channel to receive mailbox updates
	updates := make(chan client.Update, 0)
	mc.Updates = updates

	for {
		stop := make(chan struct{})
		doneIdle := make(chan error, 1)
		go func() {
			doneIdle <- mc.Idle(stop, nil)
		}()

		fmt.Println("Start idling....")
		// Listen for updates

		exit := false

		last_updates := make(chan client.MailboxUpdate, 1)

		for up := range updates {
			if exit {
				break
			}
			switch update := up.(type) {
			case *client.MailboxUpdate:
				last_updates <- *update

				exit = true
			}
		}

		close(stop)

		if err := <-doneIdle; err != nil {
			log.Fatal(fmt.Errorf("idle error: %v", err))
		}

		fmt.Println("Done idling....")

		update := <-last_updates
		fetchMessageSubject(mc, update)
	}

}

func fetchMessageSubject(mc *client.Client, update client.MailboxUpdate) {
	messages := make(chan *imap.Message, 1)
	doneFetch := make(chan error, 1)

	seqset := new(imap.SeqSet)
	seqset.AddNum(update.Mailbox.Messages)

	go func() {
		doneFetch <- mc.UidFetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()

	msg := <-messages
	if msg != nil && msg.Envelope != nil {
		fmt.Printf("msg.Envelope=%v\n", msg.Envelope)

		header := fmt.Sprintf("Unread Mail (%d)", update.Mailbox.Recent)

		body := fmt.Sprintf("[%s]\\n%s", msg.Envelope.From[0].Address(), msg.Envelope.Subject)

		cmd := exec.Command("notify-send", header, body, "--icon=mail-unread")
		err := cmd.Run()

		if err != nil {
			log.Fatal(err)
		}
	}

	if err := <-doneFetch; err != nil {
		log.Fatal(err)
	}
}

