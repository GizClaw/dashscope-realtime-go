package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	dashscope "github.com/GizClaw/dashscope-realtime-go"
)

func main() {
	client := dashscope.NewClient(os.Getenv("DASHSCOPE_API_KEY"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	session, err := client.Realtime.Connect(ctx, &dashscope.RealtimeConfig{
		Model: dashscope.ModelQwen35OmniFlashRealtime,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	additionalProperties := false
	err = session.UpdateSession(&dashscope.SessionConfig{
		Modalities: []string{dashscope.ModalityText},
		Tools: []dashscope.FunctionTool{{
			Type: dashscope.ToolTypeFunction,
			Function: dashscope.FunctionDefinition{
				Name:        "lookup_weather",
				Description: "Look up the weather for a city.",
				Parameters: &dashscope.JSONSchema{
					Type: "object",
					Properties: map[string]*dashscope.JSONSchema{
						"city": {Type: "string", Description: "City name"},
					},
					Required:             []string{"city"},
					AdditionalProperties: &additionalProperties,
				},
			},
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := session.AppendText("杭州天气怎么样？"); err != nil {
		log.Fatal(err)
	}
	if err := session.CreateResponse(nil); err != nil {
		log.Fatal(err)
	}

	waitingForFollowUp := false
	for event, eventErr := range session.Events() {
		if eventErr != nil {
			log.Fatal(eventErr)
		}
		switch event.Type {
		case dashscope.EventTypeResponseFunctionCallArgumentsDone:
			fmt.Printf("tool=%s arguments=%s\n", event.Name, event.Arguments)
			if err := session.SubmitFunctionCallOutput(event.CallID, `{"weather":"晴","temperature_c":25}`); err != nil {
				log.Fatal(err)
			}
			if err := session.CreateResponse(nil); err != nil {
				log.Fatal(err)
			}
			waitingForFollowUp = true
		case dashscope.EventTypeResponseTextDelta, dashscope.EventTypeResponseTranscriptDelta:
			fmt.Print(event.Delta)
		case dashscope.EventTypeResponseDone:
			if waitingForFollowUp {
				waitingForFollowUp = false
				continue
			}
			return
		}
	}
}
