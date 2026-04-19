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

type turn struct {
	User      string
	Assistant string
}

func main() {
	basePrompt := flag.String("prompt", "你好，请用一句话自我介绍", "first-round prompt")
	rounds := flag.Int("rounds", 3, "multi-turn rounds")
	demoError := flag.Bool("demo-error", false, "run a reproducible error path demo")
	probeAppendText := flag.Bool("probe-append-text", false, "probe AppendText input path (may fail on server)")
	probeCancel := flag.Bool("probe-cancel", false, "probe CancelResponse path before formal rounds")
	proModel := flag.String("pro-model", strings.TrimSpace(os.Getenv("DASHSCOPE_PRO_MODEL")), "optional pro model for mid-session switch")
	switchProAt := flag.Int("switch-pro-at", 2, "switch to pro model at this round (0 disables)")
	testHistoryEdit := flag.Bool("test-history-edit", true, "probe history rewrite ability")
	testProSettings := flag.Bool("test-pro-settings", true, "probe pro session setting switch")
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

	session, eventCh, errCh, err := connectTextSession(client, model)
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

	if *probeAppendText {
		probeAppendTextPathIsolated(client, model)
	}

	if *probeCancel {
		probeCancelPath(session, eventCh, errCh)
	}

	currentModel := model
	proModelSwitchOK := false
	proSettingSwitchOK := false
	historyRewriteOK := false

	if *testProSettings {
		ok, switchErr := applyProSettings(session, eventCh, errCh)
		if switchErr != nil {
			log.Printf("[probe] pro settings switch failed: %v", switchErr)
		} else {
			proSettingSwitchOK = ok
			log.Printf("[probe] pro settings switch result: %v", ok)
		}
	}

	history := make([]turn, 0, *rounds)

	for round := 1; round <= *rounds; round++ {
		if *switchProAt > 0 && round == *switchProAt && strings.TrimSpace(*proModel) != "" && strings.TrimSpace(*proModel) != currentModel {
			if err := session.Close(); err != nil {
				log.Fatalf("close before pro switch failed: %v", err)
			}

			session, eventCh, errCh, err = connectTextSession(client, strings.TrimSpace(*proModel))
			if err != nil {
				log.Fatalf("switch to pro model failed: %v", err)
			}
			defer session.Close()
			currentModel = strings.TrimSpace(*proModel)
			proModelSwitchOK = true
			log.Printf("[probe] switched model to pro: %s", currentModel)
		}

		prompt := buildRoundPrompt(round, *basePrompt, history)
		instruction := buildInstructionWithHistory(prompt, history)

		if err := session.CreateResponse(&dashscope.ResponseCreateOptions{
			Instructions: instruction,
			Modalities:   []string{dashscope.ModalityText},
		}); err != nil {
			log.Fatalf("round %d response.create failed: %v", round, err)
		}

		text, usage, collectErr := collectTextResponse(eventCh, errCh, 60*time.Second, 20*time.Second)
		if collectErr != nil {
			log.Fatalf("round %d collect response failed: %v", round, collectErr)
		}

		fmt.Printf("\n[round %d][model=%s]\n", round, currentModel)
		fmt.Printf("user: %s\nassistant: %s\n", prompt, strings.TrimSpace(text))
		if usage != nil {
			log.Printf("[round %d] token usage: input=%d output=%d total=%d", round, usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
		}

		history = append(history, turn{User: prompt, Assistant: strings.TrimSpace(text)})

		if round == 1 && *testHistoryEdit {
			ok, probeErr := probeHistoryRewrite(session, eventCh, errCh, history)
			if probeErr != nil {
				log.Printf("[probe] history rewrite failed: %v", probeErr)
			} else {
				historyRewriteOK = ok
				log.Printf("[probe] history rewrite result: %v", ok)
			}
		}
	}

	log.Printf("[summary] rounds=%d model=%s pro_model_switched=%v pro_settings_switched=%v history_rewrite=%v", *rounds, currentModel, proModelSwitchOK, proSettingSwitchOK, historyRewriteOK)
}

func runDemoError(session *dashscope.RealtimeSession) {
	err := session.AppendText("")
	if apiErr, ok := dashscope.AsError(err); ok {
		fmt.Printf("demo-error(append_text_empty): code=%s http=%d message=%s\n", apiErr.Code, apiErr.HTTPStatus, apiErr.Message)
	} else {
		fmt.Printf("demo-error(append_text_empty): type=%T message=%v\n", err, err)
	}

	err = session.SendRaw(map[string]any{})
	if apiErr, ok := dashscope.AsError(err); ok {
		fmt.Printf("demo-error(send_raw_empty): code=%s http=%d message=%s\n", apiErr.Code, apiErr.HTTPStatus, apiErr.Message)
	} else {
		fmt.Printf("demo-error(send_raw_empty): type=%T message=%v\n", err, err)
	}
}

func probeAppendTextPathIsolated(client *dashscope.Client, model string) {
	session, eventCh, errCh, err := connectTextSession(client, model)
	if err != nil {
		log.Printf("[probe] append text setup failed: %v", err)
		return
	}
	defer session.Close()
	defer func() {
		if err := session.FinishSession(); err != nil {
			log.Printf("[probe] append text finish warning: %v", err)
		}
	}()

	if err := session.AppendText("append_text 探测：请回复 append-text-ok"); err != nil {
		log.Printf("[probe] append text local error: %v", err)
		return
	}
	if err := session.CreateResponse(nil); err != nil {
		log.Printf("[probe] append text create response failed: %v", err)
		return
	}

	text, _, collectErr := collectTextResponse(eventCh, errCh, 15*time.Second, 5*time.Second)
	if collectErr != nil {
		log.Printf("[probe] append text remote result: %v", collectErr)
		return
	}
	log.Printf("[probe] append text remote output: %s", strings.TrimSpace(text))
}

func probeCancelPath(session *dashscope.RealtimeSession, eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error) {
	if err := session.CreateResponse(&dashscope.ResponseCreateOptions{
		Instructions: "cancel probe: 如果看到此消息请简单回复。",
		Modalities:   []string{dashscope.ModalityText},
	}); err != nil {
		log.Printf("[probe] cancel create response failed: %v", err)
		return
	}

	time.Sleep(120 * time.Millisecond)
	if err := session.CancelResponse(); err != nil {
		log.Printf("[probe] cancel response failed: %v", err)
		return
	}

	_, _, _ = collectTextResponse(eventCh, errCh, 6*time.Second, 2*time.Second)
	log.Printf("[probe] cancel response probe done")
}

func connectTextSession(client *dashscope.Client, model string) (*dashscope.RealtimeSession, <-chan *dashscope.RealtimeEvent, <-chan error, error) {
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
		Modalities: []string{dashscope.ModalityText},
	}); err != nil {
		session.Close()
		return nil, nil, nil, fmt.Errorf("session.update failed: %w", err)
	}

	if _, err := waitForEventType(eventCh, errCh, 5*time.Second, dashscope.EventTypeSessionUpdated); err != nil {
		log.Printf("[warn] session.updated not observed: %v", err)
	}

	return session, eventCh, errCh, nil
}

func applyProSettings(session *dashscope.RealtimeSession, eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error) (bool, error) {
	temp := 0.2
	maxTokens := 320
	if err := session.UpdateSession(&dashscope.SessionConfig{
		Modalities:      []string{dashscope.ModalityText},
		Instructions:    "[PRO MODE] 回答要更结构化，并给出关键要点。",
		Temperature:     &temp,
		MaxOutputTokens: &maxTokens,
	}); err != nil {
		return false, err
	}

	if _, err := waitForEventType(eventCh, errCh, 6*time.Second, dashscope.EventTypeSessionUpdated); err != nil {
		log.Printf("[warn] pro settings applied but session.updated not observed: %v", err)
	}

	return true, nil
}

func probeHistoryRewrite(session *dashscope.RealtimeSession, eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error, history []turn) (bool, error) {
	if len(history) == 0 {
		return false, nil
	}

	rewritten := append([]turn(nil), history...)
	rewritten[0].User = rewritten[0].User + "（已在客户端改写历史）"

	messages := make([]map[string]any, 0, len(rewritten)*2+1)
	for _, h := range rewritten {
		messages = append(messages,
			map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": h.User},
				},
			},
			map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": h.Assistant},
				},
			},
		)
	}
	messages = append(messages, map[string]any{
		"role": "user",
		"content": []map[string]any{
			{"type": "text", "text": "如果你收到了被改写的历史，请回答 history-rewrite-ok"},
		},
	})

	event := map[string]any{
		"event_id": fmt.Sprintf("event_hist_%d", time.Now().UnixNano()),
		"type":     dashscope.EventTypeResponseCreate,
		"messages": messages,
		"response": map[string]any{
			"modalities": []string{dashscope.ModalityText},
		},
	}

	if err := session.SendRaw(event); err != nil {
		return false, err
	}

	text, _, err := collectTextResponse(eventCh, errCh, 25*time.Second, 8*time.Second)
	if err != nil {
		return false, err
	}

	return strings.Contains(strings.ToLower(text), "history") || strings.Contains(strings.ToLower(text), "ok"), nil
}

func buildRoundPrompt(round int, firstPrompt string, history []turn) string {
	if round == 1 {
		return firstPrompt
	}
	if round == 2 {
		return "请根据上一轮回答，补充一个具体应用场景。"
	}
	if round == 3 {
		return "请把前两轮内容压缩成 3 条要点。"
	}
	if len(history) > 0 {
		return fmt.Sprintf("继续多轮对话，第 %d 轮，请对上一轮做简短追问并给出建议。", round)
	}
	return fmt.Sprintf("继续第 %d 轮对话。", round)
}

func buildInstructionWithHistory(prompt string, history []turn) string {
	b := strings.Builder{}
	b.WriteString("你是一个简洁的中文助手。\n")
	if len(history) > 0 {
		b.WriteString("历史对话：\n")
		for i, h := range history {
			b.WriteString(fmt.Sprintf("%d) 用户：%s\n", i+1, h.User))
			b.WriteString(fmt.Sprintf("   助手：%s\n", h.Assistant))
		}
	}
	b.WriteString("当前用户输入：")
	b.WriteString(prompt)
	b.WriteString("\n请直接回答，不要输出多余前缀。")
	return b.String()
}

func collectTextResponse(eventCh <-chan *dashscope.RealtimeEvent, errCh <-chan error, overallTimeout, idleTimeout time.Duration) (string, *dashscope.UsageStats, error) {
	var text strings.Builder
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
					return text.String(), usage, nil
				}
				return "", usage, fmt.Errorf("event channel closed unexpectedly")
			}
			if event == nil {
				continue
			}
			resetIdle()

			switch event.Type {
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
				if event.FinishReason != "" && event.FinishReason != "null" {
					return text.String(), usage, nil
				}
			case dashscope.EventTypeResponseDone:
				if event.Usage != nil {
					usage = event.Usage
				}
				return text.String(), usage, nil
			case dashscope.EventTypeError:
				if event.Error != nil {
					return text.String(), usage, fmt.Errorf("remote error: [%s] %s", event.Error.Code, event.Error.Message)
				}
				return text.String(), usage, fmt.Errorf("remote error: unknown")
			}
		case err, ok := <-errCh:
			if ok && err != nil {
				return text.String(), usage, err
			}
			if hasOutput {
				return text.String(), usage, nil
			}
			return "", usage, fmt.Errorf("error channel closed unexpectedly")
		case <-idle.C:
			if hasOutput {
				return text.String(), usage, nil
			}
			return "", usage, fmt.Errorf("idle timeout without output")
		case <-overall.C:
			if hasOutput {
				return text.String(), usage, nil
			}
			return "", usage, fmt.Errorf("overall timeout without output")
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
