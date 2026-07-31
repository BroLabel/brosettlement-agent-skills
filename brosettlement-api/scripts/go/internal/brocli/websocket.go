package brocli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BroLabel/brosettlement-agent-skills/brosettlement-api/scripts/go/internal/broauth"
	"github.com/gorilla/websocket"
)

const defaultWebSocketURL = "wss://brosettlement-staging-api.brolabel.io/v1/ws"

func runWebSocket(args []string, stdout, stderr io.Writer) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "Usage: brosettlement websocket listen [--log-path FILE] [--stop-after DURATION]")
		return errHelp
	}
	if len(args) == 0 || strings.ToLower(args[0]) != "listen" {
		return fmt.Errorf("usage: brosettlement websocket listen [options]")
	}
	flags := flag.NewFlagSet("websocket listen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	wsURL := flags.String("ws-url", defaultWebSocketURL, "BroSettlement WebSocket URL")
	logPath := flags.String("log-path", "brosettlement_ws_listener.log", "JSONL log path")
	reconnectDelay := flags.Duration("reconnect-delay", 5*time.Second, "Reconnect delay")
	stopAfter := flags.Duration("stop-after", 0, "Optional smoke-test duration")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected WebSocket arguments: %s", strings.Join(flags.Args(), " "))
	}

	logFile, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logFile.Close()
	logger := io.MultiWriter(stdout, logFile)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *stopAfter > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, *stopAfter)
		defer timeoutCancel()
	}
	writeLog(logger, "listener_started", map[string]interface{}{"wsUrl": *wsURL, "logPath": *logPath})
	for ctx.Err() == nil {
		signedURL, err := broauth.SignedWebSocketURL(*wsURL)
		if err != nil {
			return fmt.Errorf("sign WebSocket URL: %w", err)
		}
		writeLog(logger, "connecting", map[string]interface{}{"wsUrl": *wsURL})
		connection, response, err := websocket.DefaultDialer.DialContext(ctx, signedURL, nil)
		if err != nil {
			fields := map[string]interface{}{"error": err.Error()}
			if response != nil {
				fields["statusCode"] = response.StatusCode
				response.Body.Close()
			}
			writeLog(logger, "connect_failed", fields)
			if !wait(ctx, *reconnectDelay) {
				break
			}
			continue
		}
		writeLog(logger, "connected", nil)
		connectionCtx, closeConnection := context.WithCancel(ctx)
		go func() {
			<-connectionCtx.Done()
			_ = connection.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "listener stopping"),
				time.Now().Add(time.Second))
			_ = connection.Close()
		}()
		for ctx.Err() == nil {
			messageType, message, readErr := connection.ReadMessage()
			if readErr != nil {
				writeLog(logger, "connection_closed", map[string]interface{}{"error": readErr.Error()})
				break
			}
			writeLog(logger, "message", normalizeMessage(messageType, message))
		}
		closeConnection()
		_ = connection.Close()
		if ctx.Err() == nil && !wait(ctx, *reconnectDelay) {
			break
		}
	}
	writeLog(logger, "listener_stopped", map[string]interface{}{"reason": contextReason(ctx)})
	return nil
}

func normalizeMessage(messageType int, message []byte) map[string]interface{} {
	if messageType == websocket.BinaryMessage {
		return map[string]interface{}{"frameType": "binary", "base64": base64.StdEncoding.EncodeToString(message)}
	}
	var parsed interface{}
	if json.Unmarshal(message, &parsed) == nil {
		return map[string]interface{}{"frameType": "text", "json": parsed, "raw": string(message)}
	}
	return map[string]interface{}{"frameType": "text", "raw": string(message)}
}

func writeLog(writer io.Writer, event string, fields map[string]interface{}) {
	record := map[string]interface{}{"ts": time.Now().UTC().Format(time.RFC3339Nano), "event": event}
	for key, value := range fields {
		record[key] = value
	}
	encoded, _ := json.Marshal(record)
	fmt.Fprintln(writer, string(encoded))
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func contextReason(ctx context.Context) string {
	if ctx.Err() == context.DeadlineExceeded {
		return "stop_after"
	}
	return "signal"
}
