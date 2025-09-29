package ws

import (
	"bytes"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tuan-dd/go-common/response"
)

type (
	Client struct {
		b      *Board
		unique string
		conn   *websocket.Conn

		MsgBytes chan []byte
		Done     chan struct{}
	}

	RunIf[T any] interface {
		ReadProcess(data T) *response.AppError
	}
)

func (c *Client) Unique() string {
	return c.unique
}

func (c *Client) Conn() *websocket.Conn {
	return c.conn
}

func (c *Client) Process(r func(data []byte) *response.AppError) *response.AppError {
	if c == nil || r == nil {
		return response.ServerError("Client or process is nil")
	}

	go func() {
		err := c.WritePump()
		if err != nil {
			log.Printf("error in WritePump: %v", err.Error())
			return
		}
	}()

	go func() {
		err := c.ReadPump(r)
		if err != nil {
			log.Printf("error in ReadPump: %v", err.Error())
			return
		}
	}()

	return nil
}

func (c *Client) WritePump() *response.AppError {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		c.b.RemoveClient(c)
	}()

	for {
		select {
		case <-ticker.C:
			if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return nil
			}
		case msg, ok := <-c.MsgBytes:
			if !ok {
				err := c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					return response.ServerError("Failed to write close message: " + err.Error())
				}
				return nil
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return response.ServerError("Failed to write message: " + err.Error())
			}

			// TODO
			// w, err := c.conn.NextWriter(websocket.TextMessage)
			// if err != nil {
			// 	return response.ServerError(fmt.Sprintf("Failed to get next writer: %v", err))
			// }

			// _, err = w.Write(msg)
			// if err != nil {
			// 	return response.ServerError(fmt.Sprintf("Failed to write message: %v", err))
			// }

			// // Add queued chat messages to the current websocket message.
			// n := len(c.MsgBytes)

			// if n > 0 {
			// 	for range n {
			// 		_, err = w.Write(newline)
			// 		if err != nil {
			// 			return response.ServerError(fmt.Sprintf("Failed to write newline: %v", err))
			// 		}
			// 		msg = <-c.MsgBytes
			// 		_, err = w.Write(msg)
			// 		if err != nil {
			// 			return response.ServerError(fmt.Sprintf("Failed to write message: %v", err))
			// 		}
			// 	}
			// }
			// err = w.Close()
			// if err != nil {
			// 	return response.ServerError(fmt.Sprintf("Failed to close writer: %v", err.Error()))
			// }
		}
	}
}

func (c *Client) ReadPump(r func(data []byte) *response.AppError) *response.AppError {
	var errApp *response.AppError
	defer func() {
		c.b.RemoveClient(c)
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	if c.b.pongHandler != nil {
		c.conn.SetPongHandler(func(appData string) error {
			err := c.b.pongHandler(appData)
			if err != nil {
				return err
			}
			return c.conn.SetReadDeadline(time.Now().Add(pongWait))
		})
	} else {
		c.conn.SetPongHandler(func(appData string) error {
			return c.conn.SetReadDeadline(time.Now().Add(pongWait))
		})
	}
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error websocket ReadPump: %v", err)
			}
			break
		}
		_ = bytes.TrimSpace(bytes.ReplaceAll(message, newline, space))
		errApp = r(message)
		if errApp != nil {
			return errApp
		}
	}
	return nil
}
