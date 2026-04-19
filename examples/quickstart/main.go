package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	dashscope "github.com/GizClaw/dashscope-realtime-go"
)

func main() {
	demoError := flag.Bool("demo-error", false, "run a reproducible error path demo")
	flag.Parse()

	baseURL := strings.TrimSpace(os.Getenv("DASHSCOPE_BASE_URL"))
	model := strings.TrimSpace(os.Getenv("DASHSCOPE_MODEL"))
	if model == "" {
		model = dashscope.ModelQwenOmniTurboRealtimeLatest
	}

	options := []dashscope.Option{}
	if baseURL != "" {
		options = append(options, dashscope.WithBaseURL(baseURL))
	}

	if *demoError {
		runDemoError(options, model)
		return
	}

	apiKey := strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY"))
	if apiKey == "" {
		log.Fatal("DASHSCOPE_API_KEY is required")
	}

	client := dashscope.NewClient(apiKey, options...)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(ctx, &dashscope.RealtimeConfig{Model: model})
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer session.Close()

	for event, eventErr := range session.Events() {
		if eventErr != nil {
			log.Fatalf("event error: %v", eventErr)
		}
		if event == nil {
			continue
		}

		log.Printf("received event: %s", event.Type)
		if event.Type == dashscope.EventTypeSessionCreated {
			log.Printf("session created: %s", session.SessionID())
			if err := session.FinishSession(); err != nil {
				log.Printf("finish session warning: %v", err)
			}
			return
		}
	}

	log.Fatal("event stream closed before session.created")
}

func runDemoError(options []dashscope.Option, model string) {
	client := dashscope.NewClient("", options...)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.Realtime.Connect(ctx, &dashscope.RealtimeConfig{Model: model})
	if err == nil {
		fmt.Println("demo-error: unexpected success")
		return
	}

	if apiErr, ok := dashscope.AsError(err); ok {
		fmt.Printf("demo-error: type=*dashscope.Error code=%s http=%d message=%s\n", apiErr.Code, apiErr.HTTPStatus, apiErr.Message)
		return
	}

	fmt.Printf("demo-error: type=%T message=%v\n", err, err)
}
