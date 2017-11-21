// Package discovery provides a discovery service for chromecast devices
package discovery

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/barnybug/go-cast"
	"github.com/barnybug/go-cast/log"
	"github.com/hashicorp/mdns"
)

type Service struct {
	found     chan *cast.Client
	entriesCh chan *mdns.ServiceEntry

	stopPeriodic chan struct{}
}

func NewService(ctx context.Context) *Service {
	s := &Service{
		found:     make(chan *cast.Client),
		entriesCh: make(chan *mdns.ServiceEntry, 10),
	}

	go s.listener(ctx)
	return s
}

func (d *Service) Run(ctx context.Context, interval time.Duration) error {
	mdns.Query(&mdns.QueryParam{
		Service: "_googlecast._tcp",
		Domain:  "local",
		Timeout: interval,
		Entries: d.entriesCh,
	})

	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			mdns.Query(&mdns.QueryParam{
				Service: "_googlecast._tcp",
				Domain:  "local",
				Timeout: time.Second * 3,
				Entries: d.entriesCh,
			})
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (d *Service) Stop() {
	if d.stopPeriodic != nil {
		close(d.stopPeriodic)
		d.stopPeriodic = nil
	}
}

func (d *Service) Found() chan *cast.Client {
	return d.found
}

func (d *Service) listener(ctx context.Context) {
	for entry := range d.entriesCh {
		name := strings.Split(entry.Name, "._googlecast")
		// Skip everything that doesn't have googlecast in the fdqn
		if len(name) < 2 {
			continue
		}

		log.Printf("New entry: %#v\n", entry)
		client := cast.NewClient(entry.AddrV4, entry.Port)
		info := decodeTxtRecord(entry.Info)
		client.SetName(info["fn"])
		client.SetInfo(info)

		select {
		case d.found <- client:
		case <-time.After(time.Second):
		case <-ctx.Done():
			break
		}
	}
}

func decodeDnsEntry(text string) string {
	text = strings.Replace(text, `\.`, ".", -1)
	text = strings.Replace(text, `\ `, " ", -1)

	re := regexp.MustCompile(`([\\][0-9][0-9][0-9])`)
	text = re.ReplaceAllStringFunc(text, func(source string) string {
		i, err := strconv.Atoi(source[1:])
		if err != nil {
			return ""
		}

		return string([]byte{byte(i)})
	})

	return text
}

func decodeTxtRecord(txt string) map[string]string {
	m := make(map[string]string)

	s := strings.Split(txt, "|")
	for _, v := range s {
		s := strings.Split(v, "=")
		if len(s) == 2 {
			m[s[0]] = s[1]
		}
	}

	return m
}
