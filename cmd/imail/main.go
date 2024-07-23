package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"syscall"

	"os/exec"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

func usage() {
	fmt.Println(`usage: imail <command> [parameters]
    comands: setup, run`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "setup":
		setup()
		return
	case "run":
		run()
	}

}

func setup() {

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter mail: ")
	scanner.Scan()

	mail := scanner.Text()

	fmt.Print("Password: ")
	bytepw, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		os.Exit(1)
	}
	pass := string(bytepw)

	err = keyring.Set("goimail", mail, pass)

	if err != nil {
		log.Fatal(err)
	}
}

func run() {

	// get password
	secret, err := keyring.Get("goimail", "fmanuelganga@gmail.com")
	if err != nil {
		log.Fatal(err)
	}

	// TODO: this probably could be configurable for now only support gmail
	mc, err := client.DialTLS("imap.gmail.com:993", nil)

	if err != nil {
		panic(err)
	}

	// TODO: dont have this hardcoded
	err = mc.Login("fmanuelganga@gmail.com", secret)

	if err != nil {
		log.Fatalf("Login Error: %s", err.Error())
	}

	defer mc.Logout()

	if _, err := mc.Select("INBOX", false); err != nil {
		log.Fatal(err)
	}

	// Create a channel to receive mailbox updates
	updates := make(chan client.Update)
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
		last_updates := make(chan client.MailboxUpdate)

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
			log.Fatal(err)
		}

		fmt.Println("Done idling....")

		update := <-last_updates

		fetchMessageAndNotify(mc, update)
	}
}

func fetchMessageAndNotify(mc *client.Client, update client.MailboxUpdate) {
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

