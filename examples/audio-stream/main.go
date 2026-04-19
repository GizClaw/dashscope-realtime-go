package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	dashscope "github.com/GizClaw/dashscope-realtime-go"
)

type audioRound struct {
	Transcript string
	AudioBytes int
	Usage      *dashscope.UsageStats
}

func main() {
	filePath := flag.String("audio", "", "path to PCM16 mono audio file (16kHz)")
	rounds := flag.Int("rounds", 3, "multi-turn rounds")
	demoError := flag.Bool("demo-error", false, "run a reproducible error path demo")
	probeAPIMethods := flag.Bool("probe-api-methods", false, "probe additional public methods in a minimal runnable branch")
	probeCancel := flag.Bool("probe-cancel", true, "when probe-api-methods is enabled, also exercise CancelResponse")
	probeImage := flag.Bool("probe-image", false, "when probe-api-methods is enabled, also exercise AppendImage")
	testHistoryEdit := flag.Bool("test-history-edit", true, "probe history rewrite ability")
	testProSettings := flag.Bool("test-pro-settings", true, "probe pro settings switch")
	flag.Parse()

	if *rounds <= 0 {
		log.Fatal("rounds must be > 0")
	}

	apiKey := strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY"))
	if apiKey == "" {
		log.Fatal("DASHSCOPE_API_KEY is required")
	}

	baseURL := strings.TrimSpace(os.Getenv("DASHSCOPE_BASE_URL"))
	model := strings.TrimSpace(os.Getenv("DASHSCOPE_MODEL"))
	if model == "" {
		model = dashscope.ModelQwenOmniTurboRealtimeLatest
	}

	options := []dashscope.Option{}
	if baseURL != "" {
		options = append(options, dashscope.WithBaseURL(baseURL))
	}

	client := dashscope.NewClient(apiKey, options...)
	audio, err := readAudioInput(*filePath)
	if err != nil {
		log.Fatalf("load audio failed: %v", err)
	}

	session, eventCh, errCh, err := connectAudioSession(client, model)
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer session.Close()
	defer func() {
		if err := session.FinishSession(); err != nil {
			log.Printf("finish session warning: %v", err)
		}
	}()

	if *demoError {
		runDemoError(session)
		return
	}

	if *probeAPIMethods {
		runAPIMethodProbe(session, eventCh, errCh, audio, *probeCancel, *probeImage)
	}

	historyProbeOK := false
	proSettingSwitchOK := false

	roundsOut := make([]audioRound, 0, *rounds)
	for round := 1; round <= *rounds; round++ {
		if err := session.ClearInput(); err != nil {
			log.Fatalf("round %d clear input failed: %v", round, err)
		}

		if *testProSettings && round == 2 {
			ok, switchErr := applyProAudioSettings(session, eventCh, errCh)
			if switchErr != nil {
				log.Printf("[probe] round %d pro settings switch failed: %v", round, switchErr)
			} else {
				proSettingSwitchOK = ok
				log.Printf("[probe] round %d pro settings switch result: %v", round, ok)
			}
		}

		if err := sendAudioInChunks(session, audio, true); err != nil {
			log.Fatalf("round %d append audio failed: %v", round, err)
		}

		if err := session.CommitAudio(); err != nil {
			log.Fatalf("round %d commit input failed: %v", round, err)
		}
		if err := session.CreateResponse(nil); err != nil {
			log.Fatalf("round %d response.create failed: %v", round, err)
		}

		text, audioBytes, usage, collectErr := collectAudioResponse(eventCh, errCh, 90*time.Second, 20*time.Second)
		if collectErr != nil {
			log.Fatalf("round %d collect response failed: %v", round, collectErr)
		}

		roundsOut = append(roundsOut, audioRound{Transcript: strings.TrimSpace(text), AudioBytes: audioBytes, Usage: usage})
		fmt.Printf("\n[round %d]\ntranscript: %s\naudio-bytes: %d\n", round, strings.TrimSpace(text), audioBytes)
		if usage != nil {
			log.Printf("[round %d] token usage: input=%d output=%d total=%d", round, usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
		}

		if round == 1 && *testHistoryEdit {
			ok, probeErr := probeAudioHistoryRewrite(session, eventCh, errCh, roundsOut)
			if probeErr != nil {
				log.Printf("[probe] audio history rewrite failed: %v", probeErr)
			} else {
				historyProbeOK = ok
				log.Printf("[probe] audio history rewrite result: %v", ok)
			}
		}
	}

	log.Printf("[summary] rounds=%d history_rewrite=%v pro_settings_switched=%v", *rounds, historyProbeOK, proSettingSwitchOK)
}

func runDemoError(session *dashscope.RealtimeSession) {
	err := session.AppendAudio(nil)
	if apiErr, ok := dashscope.AsError(err); ok {
		fmt.Printf("demo-error(append_audio_nil): code=%s http=%d message=%s\n", apiErr.Code, apiErr.HTTPStatus, apiErr.Message)
	} else {
		fmt.Printf("demo-error(append_audio_nil): type=%T message=%v\n", err, err)
	}

	err = session.AppendAudioBase64("")
	if apiErr, ok := dashscope.AsError(err); ok {
		fmt.Printf("demo-error(append_audio_base64_empty): code=%s http=%d message=%s\n", apiErr.Code, apiErr.HTTPStatus, apiErr.Message)
	} else {
		fmt.Printf("demo-error(append_audio_base64_empty): type=%T message=%v\n", err, err)
	}
}

func runAPIMethodProbe(session *dashscope.RealtimeSession, eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error, audio []byte, probeCancel, probeImage bool) {
	if len(audio) >= 320 {
		if err := session.AppendAudioBase64(base64.StdEncoding.EncodeToString(audio[:320])); err != nil {
			log.Printf("[probe] append audio base64 failed: %v", err)
		}
	}

	if probeImage {
		if err := session.AppendImage([]byte{0xFF, 0xD8, 0xFF, 0xD9}); err != nil {
			log.Printf("[probe] append image failed: %v", err)
		}
	}

	if err := session.CommitAudio(); err != nil {
		log.Printf("[probe] commit audio failed: %v", err)
		return
	}
	if err := session.CreateResponse(nil); err != nil {
		log.Printf("[probe] create response failed: %v", err)
		return
	}

	if probeCancel {
		time.Sleep(120 * time.Millisecond)
		if err := session.CancelResponse(); err != nil {
			log.Printf("[probe] cancel response failed: %v", err)
		}
	}

	_, _, _, _ = collectAudioResponse(eventCh, errCh, 8*time.Second, 2*time.Second)

	if err := session.ClearInput(); err != nil {
		log.Printf("[probe] clear input failed: %v", err)
	}

	log.Printf("[probe] api methods probe done")
}

func connectAudioSession(client *dashscope.Client, model string) (*dashscope.RealtimeSession, <-chan *dashscope.RealtimeEvent, <-chan error, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(ctx, &dashscope.RealtimeConfig{Model: model})
	if err != nil {
		return nil, nil, nil, err
	}

	eventCh, errCh := streamEvents(session)
	if _, err := waitForEventType(eventCh, errCh, 10*time.Second, dashscope.EventTypeSessionCreated); err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("wait session.created failed: %w", err)
	}

	if err := session.UpdateSession(&dashscope.SessionConfig{
		Modalities:        []string{dashscope.ModalityText, dashscope.ModalityAudio},
		InputAudioFormat:  dashscope.AudioFormatPCM16,
		OutputAudioFormat: dashscope.AudioFormatPCM16,
		TurnDetection:     &dashscope.TurnDetection{Type: dashscope.VADModeDisabled},
	}); err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("session.update failed: %w", err)
	}

	if _, err := waitForEventType(eventCh, errCh, 6*time.Second, dashscope.EventTypeSessionUpdated); err != nil {
		log.Printf("[warn] session.updated not observed: %v", err)
	}

	return session, eventCh, errCh, nil
}

func applyProAudioSettings(session *dashscope.RealtimeSession, eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error) (bool, error) {
	temp := 0.15
	maxTokens := 420
	if err := session.UpdateSession(&dashscope.SessionConfig{
		Modalities:        []string{dashscope.ModalityText, dashscope.ModalityAudio},
		InputAudioFormat:  dashscope.AudioFormatPCM16,
		OutputAudioFormat: dashscope.AudioFormatPCM16,
		Voice:             dashscope.VoiceCherry,
		Instructions:      "[PRO AUDIO MODE] 请更详细地回答并保持条理。",
		Temperature:       &temp,
		MaxOutputTokens:   &maxTokens,
		TurnDetection:     &dashscope.TurnDetection{Type: dashscope.VADModeDisabled},
	}); err != nil {
		return false, err
	}

	if _, err := waitForEventType(eventCh, errCh, 6*time.Second, dashscope.EventTypeSessionUpdated); err != nil {
		log.Printf("[warn] pro audio settings applied but session.updated not observed: %v", err)
	}

	return true, nil
}

func probeAudioHistoryRewrite(session *dashscope.RealtimeSession, eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error, rounds []audioRound) (bool, error) {
	if len(rounds) == 0 {
		return false, nil
	}

	rewritten := append([]audioRound(nil), rounds...)
	rewritten[0].Transcript = rewritten[0].Transcript + "（已改写历史）"

	messages := make([]map[string]any, 0, len(rewritten)*2+1)
	for i, r := range rewritten {
		messages = append(messages,
			map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": fmt.Sprintf("第%d轮音频输入摘要", i+1)},
				},
			},
			map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": r.Transcript},
				},
			},
		)
	}
	messages = append(messages, map[string]any{
		"role": "user",
		"content": []map[string]any{
			{"type": "text", "text": "如果你收到了改写后的历史，请回复 history-audio-ok"},
		},
	})

	event := map[string]any{
		"event_id": fmt.Sprintf("event_audio_hist_%d", time.Now().UnixNano()),
		"type":     dashscope.EventTypeResponseCreate,
		"messages": messages,
		"response": map[string]any{
			"modalities": []string{dashscope.ModalityText},
		},
	}

	if err := session.SendRaw(event); err != nil {
		return false, err
	}

	text, _, _, err := collectAudioResponse(eventCh, errCh, 25*time.Second, 8*time.Second)
	if err != nil {
		return false, err
	}

	lower := strings.ToLower(text)
	return strings.Contains(lower, "history") || strings.Contains(lower, "ok"), nil
}

func sendAudioInChunks(session *dashscope.RealtimeSession, audio []byte, useBase64First bool) error {
	const chunkSize = 3200 // 100ms @ 16kHz pcm16 mono
	first := true
	for i := 0; i < len(audio); i += chunkSize {
		end := i + chunkSize
		if end > len(audio) {
			end = len(audio)
		}
		chunk := audio[i:end]
		var err error
		if useBase64First && first {
			err = session.AppendAudioBase64(base64.StdEncoding.EncodeToString(chunk))
			first = false
		} else {
			err = session.AppendAudio(chunk)
		}
		if err != nil {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

func collectAudioResponse(eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error, overallTimeout, idleTimeout time.Duration) (string, int, *dashscope.UsageStats, error) {
	var text strings.Builder
	audioBytes := 0
	var usage *dashscope.UsageStats
	hasOutput := false

	overall := time.NewTimer(overallTimeout)
	idle := time.NewTimer(idleTimeout)
	defer overall.Stop()
	defer idle.Stop()

	resetIdle := func() {
		if !idle.Stop() {
			select {
			case <-idle.C:
			default:
			}
		}
		idle.Reset(idleTimeout)
	}

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				if hasOutput {
					return text.String(), audioBytes, usage, nil
				}
				return "", audioBytes, usage, fmt.Errorf("event channel closed unexpectedly")
			}
			if event == nil {
				continue
			}
			resetIdle()

			switch event.Type {
			case dashscope.EventTypeResponseAudioDelta:
				if len(event.Audio) > 0 {
					hasOutput = true
					audioBytes += len(event.Audio)
				}
			case dashscope.EventTypeResponseTextDelta, dashscope.EventTypeResponseTranscriptDelta:
				if event.Delta != "" {
					hasOutput = true
					text.WriteString(event.Delta)
				}
			case dashscope.EventTypeChoicesResponse:
				if event.Delta != "" {
					hasOutput = true
					text.WriteString(event.Delta)
				}
				if len(event.Audio) > 0 {
					hasOutput = true
					audioBytes += len(event.Audio)
				}
				if event.FinishReason != "" && event.FinishReason != "null" {
					return text.String(), audioBytes, usage, nil
				}
			case dashscope.EventTypeResponseDone:
				if event.Usage != nil {
					usage = event.Usage
				}
				return text.String(), audioBytes, usage, nil
			case dashscope.EventTypeError:
				if event.Error != nil {
					return text.String(), audioBytes, usage, fmt.Errorf("remote error: [%s] %s", event.Error.Code, event.Error.Message)
				}
				return text.String(), audioBytes, usage, fmt.Errorf("remote error: unknown")
			}
		case err, ok := <-errCh:
			if ok && err != nil {
				return text.String(), audioBytes, usage, err
			}
			if hasOutput {
				return text.String(), audioBytes, usage, nil
			}
			return "", audioBytes, usage, fmt.Errorf("error channel closed unexpectedly")
		case <-idle.C:
			if hasOutput {
				return text.String(), audioBytes, usage, nil
			}
			return "", audioBytes, usage, fmt.Errorf("idle timeout without output")
		case <-overall.C:
			if hasOutput {
				return text.String(), audioBytes, usage, nil
			}
			return "", audioBytes, usage, fmt.Errorf("overall timeout without output")
		}
	}
}

func waitForEventType(eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error, timeout time.Duration, targetTypes ...string) (*dashscope.RealtimeEvent, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	target := map[string]struct{}{}
	for _, t := range targetTypes {
		target[t] = struct{}{}
	}

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return nil, fmt.Errorf("event channel closed")
			}
			if event == nil {
				continue
			}
			if event.Type == dashscope.EventTypeError {
				if event.Error != nil {
					return nil, fmt.Errorf("remote error: [%s] %s", event.Error.Code, event.Error.Message)
				}
				return nil, fmt.Errorf("remote error: unknown")
			}
			if _, ok := target[event.Type]; ok {
				return event, nil
			}
		case err, ok := <-errCh:
			if ok && err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("error channel closed")
		case <-timer.C:
			return nil, fmt.Errorf("timeout waiting for %v", targetTypes)
		}
	}
}

func readAudioInput(path string) ([]byte, error) {
	if strings.TrimSpace(path) != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("audio file is empty")
		}
		return data, nil
	}

	if envPath := strings.TrimSpace(os.Getenv("DASHSCOPE_AUDIO_FILE")); envPath != "" {
		data, err := os.ReadFile(envPath)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("audio file is empty")
		}
		return data, nil
	}

	// Fallback: 1 second of silence (16kHz, 16-bit PCM mono).
	return make([]byte, 16000*2), nil
}

func streamEvents(session *dashscope.RealtimeSession) (<-chan *dashscope.RealtimeEvent, <-chan error) {
	eventCh := make(chan *dashscope.RealtimeEvent, 128)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)
		for event, err := range session.Events() {
			if err != nil {
				errCh <- err
				return
			}
			eventCh <- event
		}
	}()

	return eventCh, errCh
}
