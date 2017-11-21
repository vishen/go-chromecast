package controllers

import (
	"errors"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"

	"github.com/barnybug/go-cast/api"
	"github.com/barnybug/go-cast/events"
	"github.com/barnybug/go-cast/log"
	"github.com/barnybug/go-cast/net"
)

const interval = time.Second * 5
const maxBacklog = 3

type HeartbeatController struct {
	pongs    int64
	ticker   *time.Ticker
	channel  *net.Channel
	eventsCh chan events.Event
}

var ping = net.PayloadHeaders{Type: "PING"}
var pong = net.PayloadHeaders{Type: "PONG"}

func NewHeartbeatController(conn *net.Connection, eventsCh chan events.Event, sourceId, destinationId string) *HeartbeatController {
	controller := &HeartbeatController{
		channel:  conn.NewChannel(sourceId, destinationId, "urn:x-cast:com.google.cast.tp.heartbeat"),
		eventsCh: eventsCh,
	}

	controller.channel.OnMessage("PING", controller.onPing)
	controller.channel.OnMessage("PONG", controller.onPong)

	return controller
}

func (c *HeartbeatController) onPing(_ *api.CastMessage) {
	err := c.channel.Send(pong)
	if err != nil {
		log.Errorf("Error sending pong: %s", err)
	}
}

func (c *HeartbeatController) sendEvent(event events.Event) {
	select {
	case c.eventsCh <- event:
	default:
		log.Printf("Dropped event: %#v", event)
	}
}

func (c *HeartbeatController) onPong(_ *api.CastMessage) {
	atomic.StoreInt64(&c.pongs, 0)
}

func (c *HeartbeatController) Start(ctx context.Context) error {
	if c.ticker != nil {
		c.Stop()
	}

	c.ticker = time.NewTicker(interval)
	go func() {
	LOOP:
		for {
			select {
			case <-c.ticker.C:
				if atomic.LoadInt64(&c.pongs) >= maxBacklog {
					log.Errorf("Missed %d pongs", c.pongs)
					c.sendEvent(events.Disconnected{errors.New("Ping timeout")})
					break LOOP
				}
				err := c.channel.Send(ping)
				atomic.AddInt64(&c.pongs, 1)
				if err != nil {
					log.Errorf("Error sending ping: %s", err)
					c.sendEvent(events.Disconnected{err})
					break LOOP
				}
			case <-ctx.Done():
				log.Println("Heartbeat stopped")
				break LOOP
			}
		}
	}()

	log.Println("Heartbeat started")
	return nil
}

func (c *HeartbeatController) Stop() {
	if c.ticker != nil {
		c.ticker.Stop()
		c.ticker = nil
	}
}
