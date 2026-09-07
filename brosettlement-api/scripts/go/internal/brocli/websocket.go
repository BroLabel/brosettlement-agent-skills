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

const defaultWebSocketStopAfter = 30 * time.Second

type webSocketOptions struct {
	wsURL          string
	logPath        string
	reconnectDelay time.Duration
	stopAfter      time.Duration
	follow         bool
}

func runWebSocket(args []string, stdout, stderr io.Writer) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "Usage: brosettlement websocket listen [--log-path FILE] [--stop-after DURATION | --follow]")
		fmt.Fprintln(stdout, "The listener stops after 30s by default; --follow runs until interrupted.")
		return errHelp
	}
	if len(args) == 0 || strings.ToLower(args[0]) != "listen" {
		return fmt.Errorf("usage: brosettlement websocket listen [options]")
	}
	environment, err := selectedEnvironment()
	if err != nil {
		return err
	}
	options, err := parseWebSocketOptions(args[1:], environment, stderr)
	if err != nil {
		return err
	}

	logFile, err := os.OpenFile(options.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logFile.Close()
	logger := io.MultiWriter(stdout, logFile)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if !options.follow {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, options.stopAfter)
		defer timeoutCancel()
	}
	writeLog(logger, "listener_started", map[string]interface{}{"wsUrl": options.wsURL, "logPath": options.logPath})
	for ctx.Err() == nil {
		signedURL, err := broauth.SignedWebSocketURL(options.wsURL)
		if err != nil {
			return fmt.Errorf("sign WebSocket URL: %w", err)
		}
		writeLog(logger, "connecting", map[string]interface{}{"wsUrl": options.wsURL})
		connection, response, err := websocket.DefaultDialer.DialContext(ctx, signedURL, nil)
		if err != nil {
			fields := map[string]interface{}{"error": err.Error()}
			if response != nil {
				fields["statusCode"] = response.StatusCode
				response.Body.Close()
			}
			writeLog(logger, "connect_failed", fields)
			if !wait(ctx, options.reconnectDelay) {
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
		if ctx.Err() == nil && !wait(ctx, options.reconnectDelay) {
			break
		}
	}
	writeLog(logger, "listener_stopped", map[string]interface{}{"reason": contextReason(ctx)})
	return nil
}

func parseWebSocketOptions(args []string, environment environmentEndpoints, stderr io.Writer) (webSocketOptions, error) {
	options := webSocketOptions{
		wsURL:          environment.webSocketURL,
		logPath:        "brosettlement_ws_listener.log",
		reconnectDelay: 5 * time.Second,
		stopAfter:      defaultWebSocketStopAfter,
	}
	flags := flag.NewFlagSet("websocket listen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.wsURL, "ws-url", options.wsURL, "BroSettlement WebSocket URL")
	flags.StringVar(&options.logPath, "log-path", options.logPath, "JSONL log path")
	flags.DurationVar(&options.reconnectDelay, "reconnect-delay", options.reconnectDelay, "Reconnect delay")
	flags.DurationVar(&options.stopAfter, "stop-after", options.stopAfter, "Bounded listener duration (default 30s)")
	flags.BoolVar(&options.follow, "follow", false, "Run without a time limit until interrupted")
	if err := flags.Parse(args); err != nil {
		return webSocketOptions{}, err
	}
	if flags.NArg() != 0 {
		return webSocketOptions{}, fmt.Errorf("unexpected WebSocket arguments: %s", strings.Join(flags.Args(), " "))
	}
	stopAfterSet := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == "stop-after" {
			stopAfterSet = true
		}
	})
	if options.follow && stopAfterSet {
		return webSocketOptions{}, fmt.Errorf("--follow cannot be combined with --stop-after")
	}
	if !options.follow && options.stopAfter <= 0 {
		return webSocketOptions{}, fmt.Errorf("--stop-after must be greater than zero; use --follow for an unbounded listener")
	}
	return options, nil
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
