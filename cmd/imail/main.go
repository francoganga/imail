package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"syscall"

	"os/exec"

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

	fmt.Println("Enter mail: ")
	scanner := bufio.NewScanner(os.Stdin)

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

	err = mc.Login("fmanuelganga@gmail.com", secret)

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

