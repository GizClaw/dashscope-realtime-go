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
	audioPath := flag.String("audio", strings.TrimSpace(os.Getenv("DASHSCOPE_AUDIO_FILE")), "path to a 16 kHz mono PCM16 question")
	flag.Parse()
	if strings.TrimSpace(*audioPath) == "" {
		log.Fatal("-audio or DASHSCOPE_AUDIO_FILE is required")
	}
	audio, err := os.ReadFile(*audioPath)
	if err != nil {
		log.Fatalf("read audio input: %v", err)
	}
	if len(audio) == 0 {
		log.Fatal("audio input cannot be empty")
	}

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
	if err := appendAudio(session, audio); err != nil {
		log.Fatal(err)
	}
	// Default server VAD commits the turn and creates the first response. Send
	// one second of silence so a file without trailing silence still ends cleanly.
	if err := appendAudio(session, make([]byte, 32000)); err != nil {
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

func appendAudio(session *dashscope.RealtimeSession, audio []byte) error {
	const chunkBytes = 3200 // 100 ms of 16 kHz mono PCM16.
	for offset := 0; offset < len(audio); offset += chunkBytes {
		end := min(offset+chunkBytes, len(audio))
		if err := session.AppendAudio(audio[offset:end]); err != nil {
			return fmt.Errorf("append audio at byte %d: %w", offset, err)
		}
		if end < len(audio) {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}
