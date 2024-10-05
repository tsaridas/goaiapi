package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/gorilla/websocket"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type Text string

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

func responseString(resp *genai.GenerateContentResponse) string {
	var b strings.Builder
	for i, cand := range resp.Candidates {
		if len(resp.Candidates) > 1 {
			fmt.Fprintf(&b, "%d:", i+1)
		}
		b.WriteString(contentString(cand.Content))
	}
	return b.String()
}

func contentString(c *genai.Content) string {
	var b strings.Builder
	if c == nil || c.Parts == nil {
		return ""
	}
	for i, part := range c.Parts {
		if i > 0 {
			fmt.Fprintf(&b, ";")
		}
		fmt.Fprintf(&b, "%v", part)
	}
	return b.String()
}

type Message struct {
	Content string `json:"content"`
}

func main() {
	// Get Gemini AI API key from environment variable
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatalf("GEMINI_API_KEY environment variable is not set")
	}

	// Initialize Gemini AI client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Choose a cheaper model
	model := client.GenerativeModel("gemini-1.5-flash")
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
	}

	http.HandleFunc("/ai", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer conn.Close()

		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}

			var msg Message
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Println(err)
				continue
			}

			// Start timing
			startTime := time.Now()

			// Generate response using Gemini AI
			resp, err := model.GenerateContent(ctx, genai.Text(msg.Content))
			if err != nil {
				log.Println(err)
				continue
			}
			got := responseString(resp)
			log.Println("Response", got)
			response := Message{Content: got}
			responseJSON, err := json.Marshal(response)
			if err != nil {
				log.Println(err)
				continue
			}

			if err := conn.WriteMessage(messageType, responseJSON); err != nil {
				log.Println(err)
				return
			}

			// End timing and log the duration
			duration := time.Since(startTime)
			log.Printf("Response time: %v\n", duration)
		}
	})

	http.HandleFunc("/start-chat", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer conn.Close()
		// Start a chat with the AI client using model.StartChat
		session := model.StartChat()
		log.Println("Chat started with the AI client")
		send := func(msg string, streaming bool) string {
			log.Println("sending message: ", msg)
			if msg == "" {
				return ""
			}
			nh := len(session.History)
			if streaming {
				iter := session.SendMessageStream(ctx, genai.Text(msg))
				for {
					_, err := iter.Next()
					if err == iterator.Done {
						break
					}
					if err != nil {
						log.Fatal(err)
					}
				}
			} else {
				if _, err := session.SendMessage(ctx, genai.Text(msg)); err != nil {
					log.Fatal(err)
				}
			}
			// Check that two items, the sent message and the response) were
			// added to the history.
			if g, w := len(session.History), nh+2; g != w {
				log.Printf("history length: got %d, want %d", g, w)
			}
			// Last history item is the one we just got from the model.
			return contentString(session.History[len(session.History)-1])
		}
		for {
			_, p, err := conn.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}

			var msg Message
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Println(err)
				continue
			}

			// Start timing
			startTime := time.Now()
			// Assuming session returns a string response for the chat
			chatResponse := Message{Content: send(msg.Content, true)}
			chatResponseJSON, err := json.Marshal(chatResponse)
			if err != nil {
				log.Println(err)
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, chatResponseJSON); err != nil {
				log.Println(err)
				return
			}

			// End timing and log the duration
			duration := time.Since(startTime)
			log.Printf("Response time: %v\n", duration)
		}
	})
	http.HandleFunc("/ops", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer conn.Close()
		// Start a chat with the AI client using model.StartChat
		session := model.StartChat()
		log.Println("Chat started with the AI client")
		send := func(msg string, streaming bool) string {
			if msg == "echo true" || msg == "echo false" {
				return msg
			}

			nh := len(session.History)
			if streaming {
				iter := session.SendMessageStream(ctx, genai.Text(msg))
				for {
					_, err := iter.Next()
					if err == iterator.Done {
						break
					}
					if err != nil {
						log.Fatal(err)
					}
				}
			} else {
				if _, err := session.SendMessage(ctx, genai.Text(msg)); err != nil {
					log.Fatal(err)
				}
			}
			// Check that two items, the sent message and the response) were
			// added to the history.
			if g, w := len(session.History), nh+2; g != w {
				log.Printf("history length: got %d, want %d", g, w)
			}
			// Last history item is the one we just got from the model.
			return contentString(session.History[len(session.History)-1])
		}
		send("you are connected to a bash terminal that runs on a Debian GNU/Linux 12 (bookworm).Everything you reply will be copy pasted to bash as is to be ran. Please don't reply with anything other than bash commands. if you don't know return echo false. You will be pasted the reply of the bash terminal as a response and if the task is done return echo true. Please make sure that all commands you send return and don't hang forver and are cli ready meaning that you cannot confirum. don't install any new packages unless asked.", false)
		for {
			_, p, err := conn.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}

			var msg Message
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Println(err)
				continue
			}

			// Start timing
			startTime := time.Now()
			reply := send(msg.Content, false)
			log.Println("reply", reply)
			// Assuming session returns a string response for the chat
			chatResponse := Message{Content: reply}
			// run a bash command with the reply on the system
			log.Println("running command: ", reply)
			cmd := exec.Command("bash", "-c", reply)
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Println("error running command: ", err, string(output))
				fixed_reply := send("There was an error running the command. Output was: "+string(output)+"\nFix it.", false)
				cmd := exec.Command("bash", "-c", string(fixed_reply))
				output, err := cmd.CombinedOutput()
				if err != nil {
					log.Println("error running second command: ", err, string(output), fixed_reply)
				}
				log.Println("Fixed error reply: ", string(output))

			}
			log.Println("sending back command output: ", string(output))
			chatResponse.Content = string(output)

			chatResponseJSON, err := json.Marshal(chatResponse)
			if err != nil {
				log.Println(err)
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, chatResponseJSON); err != nil {
				log.Println(err)
				return
			}

			// End timing and log the duration
			duration := time.Since(startTime)
			log.Printf("Response time: %v\n", duration)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		if r.Method == "OPTIONS" {
			return
		}
	})

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
