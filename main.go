package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"os"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-sasl"
)

type XOAUTH2Client struct {
	Username string
	Token    string
}

func (c *XOAUTH2Client) Start() (mech string, ir []byte, err error) {
	mech = "XOAUTH2"
	var str = "user=" + c.Username + "\x01"

	str += "auth=Bearer " + c.Token + "\x01\x01"
	fmt.Printf("str=%v\n", str)
	ir = []byte(str)
	return
}

func (c *XOAUTH2Client) Next(challenge []byte) ([]byte, error) {
	fmt.Printf("challenge=%v\n", string(challenge))
	authBearerErr := &sasl.OAuthBearerError{}
	if err := json.Unmarshal(challenge, authBearerErr); err != nil {
		return nil, err
	} else {
		return nil, authBearerErr
	}
}

func main() {
	// TODO: this probably could be configurable for now only support gmail
	mc, err := client.DialTLS("imap.gmail.com:993", nil)

	if err != nil {
		panic(err)
	}

	auth()

	messageChannel := make(chan string, 0)

	ctx, cancel := context.WithCancel(context.Background())

	go authServer(ctx, messageChannel)

	code := <-messageChannel

	fmt.Printf("code=%v\n", code)

	// shutdown the http server
	cancel()

	// TODO: provide the account from a config file
	xoauthClient := &XOAUTH2Client{
		Username: "fmanuelganga@gmail.com",
		Token:    code,
	}

	err = mc.Authenticate(xoauthClient)

	if err != nil {
		fmt.Printf("err=%+v\n", err)
		os.Exit(1)
	}

	_, err = mc.Select("INBOX", true)

	if err != nil {
		panic(err)
	}

	criteria := imap.SearchCriteria{
		WithFlags: []string{"\\Seen"},
		Since:     time.Now().Add(-time.Hour * 24),
	}

	res, err := mc.Search(&criteria)

	if err != nil {
		panic(err)
	}

	fmt.Printf("res=%v\n", res)

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)

	set := &imap.SeqSet{}
	set.AddNum(res[0])

	fmt.Println("fetching...")

	// TODO: research how to only fetch the message subjects
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	go func() {
		done <- mc.Fetch(set, items, messages)
	}()

	log.Println("Last message:")
	msg := <-messages
	r := msg.GetBody(section)
	if r == nil {
		log.Fatal("Server didn't returned message body")
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	m, err := mail.ReadMessage(r)
	if err != nil {
		log.Fatal(err)
	}

	header := m.Header
	log.Println("Date:", header.Get("Date"))
	log.Println("From:", header.Get("From"))
	log.Println("To:", header.Get("To"))
	log.Println("Subject:", header.Get("Subject"))

	body, err := io.ReadAll(m.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(body))
}

func authServer(ctx context.Context, messageChannel chan<- string) {

	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		//read code
		code := r.URL.Query().Get("code")

		// send the code
		messageChannel <- code
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr: ":4000",
	}

	go func() {
		fmt.Println("HTTP server is listening on :8080...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("Error starting HTTP server:", err)
			messageChannel <- "Error starting HTTP server"
		}
	}()

	<-ctx.Done()

	// Shut down the server gracefully
	fmt.Println("Shutting down the server gracefully...")
	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctxShutDown); err != nil {
		fmt.Println("Error shutting down HTTP server:", err)
	} else {
		fmt.Println("Server shut down gracefully.")
	}
}

