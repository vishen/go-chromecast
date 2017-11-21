package cast

import (
	"errors"
	"fmt"
	"net"

	"golang.org/x/net/context"

	"github.com/barnybug/go-cast/controllers"
	"github.com/barnybug/go-cast/events"
	"github.com/barnybug/go-cast/log"
	castnet "github.com/barnybug/go-cast/net"
)

type Client struct {
	name       string
	info       map[string]string
	host       net.IP
	port       int
	conn       *castnet.Connection
	ctx        context.Context
	cancel     context.CancelFunc
	heartbeat  *controllers.HeartbeatController
	connection *controllers.ConnectionController
	receiver   *controllers.ReceiverController
	media      *controllers.MediaController

	Events chan events.Event
}

const DefaultSender = "sender-0"
const DefaultReceiver = "receiver-0"
const TransportSender = "Tr@n$p0rt-0"
const TransportReceiver = "Tr@n$p0rt-0"

func NewClient(host net.IP, port int) *Client {
	return &Client{
		host:   host,
		port:   port,
		ctx:    context.Background(),
		Events: make(chan events.Event, 16),
	}
}

func (c *Client) IP() net.IP {
	return c.host
}

func (c *Client) Port() int {
	return c.port
}

func (c *Client) SetName(name string) {
	c.name = name
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) SetInfo(info map[string]string) {
	c.info = info
}

func (c *Client) Uuid() string {
	return c.info["id"]
}

func (c *Client) Device() string {
	return c.info["md"]
}

func (c *Client) Status() string {
	return c.info["rs"]
}

func (c *Client) String() string {
	return fmt.Sprintf("%s - %s:%d", c.name, c.host, c.port)
}

func (c *Client) Connect(ctx context.Context) error {
	c.conn = castnet.NewConnection()
	err := c.conn.Connect(ctx, c.host, c.port)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(c.ctx)
	c.cancel = cancel

	// start connection
	c.connection = controllers.NewConnectionController(c.conn, c.Events, DefaultSender, DefaultReceiver)
	if err := c.connection.Start(ctx); err != nil {
		return err
	}

	// start heartbeat
	c.heartbeat = controllers.NewHeartbeatController(c.conn, c.Events, TransportSender, TransportReceiver)
	if err := c.heartbeat.Start(ctx); err != nil {
		return err
	}

	// start receiver
	c.receiver = controllers.NewReceiverController(c.conn, c.Events, DefaultSender, DefaultReceiver)
	if err := c.receiver.Start(ctx); err != nil {
		return err
	}

	c.Events <- events.Connected{}

	return nil
}

func (c *Client) NewChannel(sourceId, destinationId, namespace string) *castnet.Channel {
	return c.conn.NewChannel(sourceId, destinationId, namespace)
}

func (c *Client) Close() {
	c.cancel()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *Client) Receiver() *controllers.ReceiverController {
	return c.receiver
}

func (c *Client) launchMediaApp(ctx context.Context) (string, error) {
	// get transport id
	status, err := c.receiver.GetStatus(ctx)
	if err != nil {
		return "", err
	}
	app := status.GetSessionByAppId(AppMedia)
	if app == nil {
		// needs launching
		status, err = c.receiver.LaunchApp(ctx, AppMedia)
		if err != nil {
			return "", err
		}
		app = status.GetSessionByAppId(AppMedia)
	}

	if app == nil {
		return "", errors.New("Failed to get media transport")
	}
	return *app.TransportId, nil
}

func (c *Client) IsPlaying(ctx context.Context) bool {
	status, err := c.receiver.GetStatus(ctx)
	if err != nil {
		log.Fatalln(err)
		return false
	}
	app := status.GetSessionByAppId(AppMedia)
	if app == nil {
		return false
	}
	if *app.StatusText == "Ready To Cast" {
		return false
	}
	return true
}

func (c *Client) Media(ctx context.Context) (*controllers.MediaController, error) {
	if c.media == nil {
		transportId, err := c.launchMediaApp(ctx)
		if err != nil {
			return nil, err
		}
		conn := controllers.NewConnectionController(c.conn, c.Events, DefaultSender, transportId)
		if err := conn.Start(ctx); err != nil {
			return nil, err
		}
		c.media = controllers.NewMediaController(c.conn, c.Events, DefaultSender, transportId)
		if err := c.media.Start(ctx); err != nil {
			return nil, err
		}
	}
	return c.media, nil
}
