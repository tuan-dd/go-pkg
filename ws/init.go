package ws

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tuan-dd/go-common/response"
	typeCustom "github.com/tuan-dd/go-common/type-custom"
)

type (
	Board struct {
		logger      typeCustom.LoggerWithAlert
		mu          sync.Mutex
		upgrader    websocket.Upgrader
		pongHandler func(string) error
		clients     sync.Map
	}

	Config struct {
		WriteWait         time.Duration
		PongWait          time.Duration
		MaxMessageSize    int64
		ReadBufferSize    int
		EnableCompression bool
		WriteBufferSize   int
		CheckOrigin       func(r *http.Request) bool
		PongHandler       func(string) error
	}
)

var (
	newline        = []byte{'\n'}
	space          = []byte{' '}
	writeWait      time.Duration
	pingPeriod     time.Duration
	pongWait       time.Duration
	maxMessageSize int64
)

func NewBoard(cfg *Config, logger typeCustom.LoggerWithAlert) *Board {
	writeWait = cfg.WriteWait
	pongWait = cfg.PongWait
	maxMessageSize = cfg.MaxMessageSize
	pingPeriod = (pongWait * 9) / 10

	if cfg.CheckOrigin == nil {
		cfg.CheckOrigin = func(r *http.Request) bool { return true }
	}
	b := &Board{
		logger:      logger,
		clients:     sync.Map{},
		pongHandler: cfg.PongHandler,
		upgrader: websocket.Upgrader{
			EnableCompression: true,
			CheckOrigin:       cfg.CheckOrigin,
			ReadBufferSize:    cfg.ReadBufferSize,
			WriteBufferSize:   cfg.WriteBufferSize,
		},
	}
	return b
}

func (b *Board) UpgradeWebSocket(w http.ResponseWriter, r *http.Request) (*Client, *response.AppError) {
	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, response.ServerError(fmt.Sprintf("Failed to upgrade connection to WebSocket: %v", err))
	}

	client := &Client{
		conn:     conn,
		MsgBytes: make(chan []byte, 10),
		b:        b,
		Done:     make(chan struct{}),
	}
	return client, nil
}

func (b *Board) GetClients(unique string) []*Client {
	if clients, exists := b.clients.Load(unique); exists {
		return clients.([]*Client)
	}
	return nil
}

func (b *Board) AddClient(unique string, client *Client) {
	clients, exists := b.clients.Load(unique)
	client.unique = unique
	if !exists {
		clients = []*Client{}
	}

	if len(clients.([]*Client)) >= 10 {
		clients.([]*Client)[0] = client
		return
	}

	clients = append(clients.([]*Client), client)
	b.clients.Store(unique, clients)
}

func (b *Board) RemoveClient(c *Client) {
	b.mu.Lock()
	defer b.mu.Unlock()

	select {
	case <-c.Done:
		return
	default:
		if clients, exists := b.clients.Load(c.unique); exists {
			newClients := make([]*Client, 0, 20)
			for _, cc := range clients.([]*Client) {
				if cc != c {
					newClients = append(newClients, cc)
				}
			}
			b.clients.Store(c.unique, newClients)
		}
		close(c.Done)
		close(c.MsgBytes)
		_ = c.conn.Close()
	}
}
