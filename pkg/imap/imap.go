package imap

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"

	//"github.com/jellydator/ttlcache/v3"

	"github.com/dgraph-io/ristretto"
)

type Imap struct {
	Username string
	Password string
	Host     string
	Port     int
	//Cache    *ttlcache.Cache[string, *bytes.Buffer]

	connected bool
	client    *client.Client
	lastUid   uint32
	stopCh    chan struct{}
	cache     *ristretto.Cache
}

func (i *Imap) connect() error {
	c, err := client.DialTLS(fmt.Sprintf("%s:%d", i.Host, i.Port), nil)

	if err != nil {
		return err
	}

	if err := c.Login(i.Username, i.Password); err != nil {
		return err
	}

	i.connected = true
	i.client = c
	i.stopCh = make(chan struct{}, 1)
	i.cache, _ = ristretto.NewCache(&ristretto.Config{
		BufferItems: 64,
		NumCounters: 100000,
		MaxCost:     10000,
	})

	return nil
}

func New(host string, port int, username string, password string) (*Imap, error) {

	imap := &Imap{Host: host, Port: port, Username: username, Password: password}

	err := imap.connect()

	return imap, err

}

func (i *Imap) Start(c *imap.SearchCriteria, pattern *regexp.Regexp) error {

	if !i.connected {
		err := i.connect()

		if err != nil {
			return err
		}
	}

	//i.client.SetDebug(os.Stdout)
	//pinPattern := regexp.MustCompile(`>\s+(\d{6})\s+<`)

	status, _ := i.client.Select("INBOX", true)
	i.lastUid = status.UidNext

	formattedTo := regexp.MustCompile(`([\w-\.+]+@([\w-]+\.)+[\w-]{2,})`)

	//i.lastUid = 1

	for {
		select {
		case <-i.stopCh:
			return nil
		default:

			if !i.connected {
				i.connect()
			}

			time.Sleep(5 * time.Second)

			_, err := i.client.Select("INBOX", true)

			if err != nil {
				i.connected = false
				continue
			}

			seqsetC, _ := imap.ParseSeqSet(fmt.Sprintf("%d:*", i.lastUid))
			c.Uid = seqsetC

			uids, _ := i.client.Search(c)

			if len(uids) > 0 {

				lastSeqNum := uids[len(uids)-1]

				seqset := new(imap.SeqSet)
				seqset.AddNum(uids...)

				var section imap.BodySectionName
				items := []imap.FetchItem{imap.FetchUid, section.FetchItem()}

				messages := make(chan *imap.Message, 10)
				done := make(chan error, 1)
				go func() {
					done <- i.client.Fetch(seqset, items, messages)
				}()

				for msg := range messages {

					if msg.Uid < i.lastUid {
						continue
					}

					if msg.SeqNum == lastSeqNum {
						i.lastUid = msg.Uid + 1
					}

					r := msg.GetBody(&section)

					if r == nil {
						log.Fatal("Server didn't returned message body")
					}

					mr, _ := mail.CreateReader(r)
					header := mr.Header
					toAddress, _ := header.AddressList("To")
					to := toAddress[0].String()
					toMatches := formattedTo.FindStringSubmatch(to)

					if len(toMatches) > 0 {
						to = toMatches[1]
					}

					var matches []string

					for {
						p, err := mr.NextPart()
						if err == io.EOF {
							break
						} else if err != nil {
							log.Fatal(err)
						}

						switch p.Header.(type) {
						case *mail.InlineHeader:
							b, _ := ioutil.ReadAll(p.Body)
							matches = pattern.FindStringSubmatch(string(b))

							if len(matches) != 0 {
								break
							}
						}
					}

					if len(matches) > 0 {
						buf := &bytes.Buffer{}
						gob.NewEncoder(buf).Encode(matches)
						//i.Cache.Set(strings.ToLower(to), buf, 15*time.Minute)
						i.cache.Set(strings.ToLower(to), *buf, 1)
					}

				}

				if err := <-done; err != nil {
					i.connected = false
					continue
				}

			}

		}
	}

}

func (i *Imap) Get(email string) *bytes.Buffer {

	val, found := i.cache.Get(email)

	if !found {
		return nil
	}

	buf, ok := val.(bytes.Buffer)

	if !ok {
		return nil
	}

	return &buf
}
