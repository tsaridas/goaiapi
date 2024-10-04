package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/gorilla/websocket"
)

type Message struct {
	Content string `json:"content"`
}

func main() {
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ops", nil)
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer conn.Close()

	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("Error reading:", err)
				return
			}
			var resp Message
			if err := json.Unmarshal(message, &resp); err != nil {
				log.Println("Error unmarshaling:", err, resp)
				continue
			}
			fmt.Println("AI:", resp.Content)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := Message{Content: scanner.Text()}
		message, err := json.Marshal(msg)
		if err != nil {
			log.Println("Error marshaling:", err)
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Println("Error writing:", err)
			return
		}
	}
}
