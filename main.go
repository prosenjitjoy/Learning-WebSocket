package main

import (
	"context"
	"main/socket"
	"net/http"
)

func main() {
	setupAPI()
	http.ListenAndServeTLS(":5000", "./keys/server.crt", "./keys/server.key", nil)
}

func setupAPI() {
	ctx := context.Background()
	manager := socket.NewManager(ctx)
	http.Handle("/", http.FileServer(http.Dir("./frontend")))
	http.HandleFunc("/ws", manager.ServeWS)
	http.HandleFunc("/login", manager.LoginHandler)
}
