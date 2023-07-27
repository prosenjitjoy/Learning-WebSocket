package socket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"main/auth"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const BufferSize = 1024

var upgrader = websocket.Upgrader{
	CheckOrigin:     checkOrigin,
	ReadBufferSize:  BufferSize,
	WriteBufferSize: BufferSize,
}

type Manager struct {
	Clients  ClientList
	Handlers map[string]EventHandler
	OTPs     auth.RetentionMap
	sync.RWMutex
}

func (m *Manager) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req auth.UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Username == "joy" && req.Password == "123" {
		otp := m.OTPs.NewOTP()

		resp := auth.LoginResponse{
			OTP: otp.Key,
		}

		data, err := json.Marshal((resp))
		if err != nil {
			log.Println(err)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}

	if req.Username == "test" && req.Password == "123" {
		otp := m.OTPs.NewOTP()

		resp := auth.LoginResponse{
			OTP: otp.Key,
		}

		data, err := json.Marshal((resp))
		if err != nil {
			log.Println(err)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}
}

func NewManager(ctx context.Context) *Manager {
	m := &Manager{
		Clients:  ClientList{},
		Handlers: make(map[string]EventHandler),
		OTPs:     auth.NewRetentionMap(ctx, 5*time.Second),
	}
	m.SetupEventHandlers()
	return m
}

func ChatRoomHandler(event Event, c *Client) error {
	var changeRoomEvent ChangeRoomEvent
	if err := json.Unmarshal(event.Payload, &changeRoomEvent); err != nil {
		return fmt.Errorf("bad payload in request: %v", err)
	}

	c.chatroom = changeRoomEvent.Name
	return nil

}

func (m *Manager) SetupEventHandlers() {
	m.Handlers[EventSendMessage] = SendMessage
	m.Handlers[EventChatRoom] = ChatRoomHandler
}

func SendMessage(event Event, c *Client) error {
	var chatevent SendMessageEvent
	if err := json.Unmarshal(event.Payload, &chatevent); err != nil {
		return fmt.Errorf("bad payload in request: %v", err)
	}

	var broadcastMessage NewMessageEvent
	broadcastMessage.Send = time.Now()
	broadcastMessage.Message = chatevent.Message
	broadcastMessage.From = chatevent.From

	data, err := json.Marshal(broadcastMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal broadcast message: %v", err)
	}

	outgoingEvent := Event{
		Payload: data,
		Type:    EventNewMessage,
	}

	for client := range c.manger.Clients {
		if client.chatroom == c.chatroom {

			client.egress <- outgoingEvent
		}
	}

	return nil
}

func (m *Manager) RouteEvent(event Event, c *Client) error {
	if handler, ok := m.Handlers[event.Type]; ok {
		if err := handler(event, c); err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("there is no such event type")
	}
}

func (m *Manager) ServeWS(w http.ResponseWriter, r *http.Request) {
	otp := r.URL.Query().Get("otp")
	if otp == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !m.OTPs.VerifyOTP(otp) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	log.Println("new connection")

	// upgrade regular http connection into websocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal("err upgrading http request to websocket")
	}

	client := NewClient(conn, m)
	m.AddClient(client)

	// start client processses
	go client.ReadMessages()
	go client.WriteMessages()
}

func (m *Manager) AddClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	m.Clients[client] = true
}

func (m *Manager) RemoveClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.Clients[client]; ok {
		client.connection.Close()
		delete(m.Clients, client)
	}
}

func checkOrigin(r *http.Request) bool {
	return r.Header.Get("Origin") == "https://localhost:5000"
}
