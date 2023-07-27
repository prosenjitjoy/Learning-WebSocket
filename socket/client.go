package socket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

var (
	pongWaitTime  = 10 * time.Second
	pingIntervarl = (pongWaitTime * 9) / 10
)

type ClientList map[*Client]bool

type Client struct {
	connection *websocket.Conn
	manger     *Manager
	// egress is used to avoid concurrent writes on the ewebsocket connection
	egress   chan Event
	chatroom string
}

func NewClient(conn *websocket.Conn, manager *Manager) *Client {
	return &Client{
		connection: conn,
		manger:     manager,
		egress:     make(chan Event),
	}
}

func (c *Client) ReadMessages() {
	defer func() {
		// cleanup connection
		c.manger.RemoveClient(c)
	}()

	if err := c.connection.SetReadDeadline(time.Now().Add(pongWaitTime)); err != nil {
		log.Println(err)
		return
	}

	c.connection.SetReadLimit(BufferSize)

	c.connection.SetPongHandler(c.pongHandler)

	for {
		_, payload, err := c.connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("error reading message:", err)
			}
			break
		}

		var request Event
		if err := json.Unmarshal(payload, &request); err != nil {
			log.Println("error marshalling event:", err)
			break
		}

		if err := c.manger.RouteEvent(request, c); err != nil {
			log.Println("error routing event:", err)
		}
	}
}

func (c *Client) WriteMessages() {
	defer func() {
		// cleanup connection
		c.manger.RemoveClient(c)
	}()

	ticker := time.NewTicker(pingIntervarl)

	for {
		select {
		case message, ok := <-c.egress:
			if !ok {
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					log.Println("connection closed:", err)
				}
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				log.Println("error marshaling to json:", err)
				return
			}

			if err := c.connection.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Println("failed to send message", err)
			}
			log.Println("message sent")
		case <-ticker.C:
			log.Println("ping")

			// send a ping to the client
			if err := c.connection.WriteMessage(websocket.PingMessage, []byte(``)); err != nil {
				log.Println("write message error:", err)
				return
			}
		}
	}
}

func (c *Client) pongHandler(pongMsg string) error {
	log.Println("pong")
	return c.connection.SetReadDeadline(time.Now().Add(pongWaitTime))
}
