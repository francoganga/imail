package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"os/exec"
	"os/signal"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/gookit/goutil/dump"
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

func process(mc *client.Client) {
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
			log.Fatal(err)
		}

		fmt.Println("Done idling....")

		update := <-last_updates

		fetchMessageAndNotify(mc, update)
	}
}

func makeTestClient() *client.Client {
	mc, err := client.Dial("localhost:143")
	if err != nil {
		log.Fatal(err)
	}

	err = mc.Login("ubuntu", "asdfqwer")
	if err != nil {
		log.Fatal(err)
	}

	return mc
}

func makeClient() *client.Client {
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

	return mc
}

func run() {

	mc := makeClient()

	defer mc.Logout()

	_, err := mc.Select("INBOX", true)

	if err != nil {
		log.Fatal(err)
	}

	// TODO: config flag for this
	ticker := time.NewTicker(10 * time.Minute)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				messages := fetchUnseenMessages(mc)
				notifyMessages(messages)
			}
		}
	}()

	exitSignal := make(chan os.Signal)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)
	<-exitSignal

	ticker.Stop()
	done <- true
	fmt.Println("Application Stopped")

}

func notifyMessages(messages []*imap.Message) {

	header := fmt.Sprintf("Unread Mail (%d)", len(messages))

	msg := messages[len(messages)-1]

	body := fmt.Sprintf("[%s]\\n%s", msg.Envelope.From[0].Address(), msg.Envelope.Subject)

	cmd := exec.Command("notify-send", header, body, "--icon=mail-unread")
	err := cmd.Run()

	if err != nil {
		log.Fatal(err)
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

func fetchMessage(mc *client.Client, uid uint32) {
	messages := make(chan *imap.Message, 1)
	doneFetch := make(chan error, 1)

	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)

	go func() {
		doneFetch <- mc.UidFetch(seqset, []imap.FetchItem{imap.FetchBody, imap.FetchEnvelope}, messages)
	}()

	msg := <-messages
	if msg != nil {
		dump.P(msg)
	} else {
		log.Fatal("msg is nil")
	}

	if err := <-doneFetch; err != nil {
		log.Fatal(err)
	}
}

func listMailboxes(mc *client.Client) {
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)

	go func() {
		done <- mc.List("", "*", mailboxes)
	}()

	for m := range mailboxes {
		fmt.Println(m.Name)
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}
}

func fetchUnseenMessages(mc *client.Client) []*imap.Message {
	criteria := &imap.SearchCriteria{
		WithoutFlags: []string{"\\Seen"},
	}

	set, err := mc.Search(criteria)

	if err != nil {
		log.Fatal(err)
	}

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	seqset := new(imap.SeqSet)
	seqset.AddNum(set...)

	go func() {
		done <- mc.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()

	res := make([]*imap.Message, 0)

	for msg := range messages {
		res = append(res, msg)
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	return res
}

