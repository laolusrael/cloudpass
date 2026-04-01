package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"cloudpass/internal/multipass"

	"github.com/coder/websocket"
	wsjson "github.com/coder/websocket/wsjson"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type TerminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

type TerminalHandler struct {
	client     multipass.Client
	sshKeyPath string
}

func NewTerminalHandler(client multipass.Client, sshKeyPath string) *TerminalHandler {
	return &TerminalHandler{client: client, sshKeyPath: sshKeyPath}
}

func (h *TerminalHandler) HandleTerminal(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "instance name is required",
		})
	}

	instance, err := h.client.GetInstance(name)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "instance not found",
		})
	}

	if instance.State != "Running" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "instance is not running",
		})
	}

	ip, err := h.client.GetInstanceIP(name)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "instance has no IP address",
		})
	}

	conn, err := websocket.Accept(c.Response(), c.Request(), nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to accept websocket")
		return err
	}
	defer conn.CloseNow()

	log.Debug().Str("instance", name).Str("ip", ip).Msg("WebSocket terminal connection started")

	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()

	sshClient := multipass.NewSSHClient(30)
	if err := sshClient.Connect(h.sshKeyPath, ip, 30); err != nil {
		log.Error().Err(err).Str("ip", ip).Msg("failed to SSH connect")
		wsjson.Write(ctx, conn, map[string]string{
			"type": "error",
			"data": fmt.Sprintf("failed to connect via SSH: %v", err),
		})
		return nil
	}
	defer sshClient.Close()

	log.Debug().Str("ip", ip).Msg("SSH connection established")

	session, err := sshClient.OpenTerminal(24, 80)
	if err != nil {
		log.Error().Err(err).Msg("failed to open terminal session")
		wsjson.Write(ctx, conn, map[string]string{
			"type": "error",
			"data": fmt.Sprintf("failed to open terminal: %v", err),
		})
		return nil
	}
	defer session.Close()

	log.Debug().Msg("Terminal session opened")

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Error().Err(err).Msg("failed to get stdin pipe")
		return nil
	}
	defer stdin.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Error().Err(err).Msg("failed to get stdout pipe")
		return nil
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		log.Error().Err(err).Msg("failed to get stderr pipe")
		return nil
	}

	if err := session.Shell(); err != nil {
		log.Error().Err(err).Msg("failed to start shell")
		return nil
	}

	var wg sync.WaitGroup

	// Handle WebSocket messages - distinguish between text (resize) and binary (terminal data)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Debug().Err(err).Msg("websocket read error")
				return
			}

			if typ == websocket.MessageText {
				// Parse as JSON for resize events
				var msg TerminalMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					log.Debug().Err(err).Msg("failed to parse JSON message")
					continue
				}

				if msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
					log.Debug().Int("cols", msg.Cols).Int("rows", msg.Rows).Msg("Received resize event")
					err := session.WindowChange(msg.Rows, msg.Cols)
					if err != nil {
						log.Warn().Err(err).Msg("failed to resize terminal")
					}
				}
			}
			// Binary messages are handled by stdin copy goroutine
		}
	}()

	// Stream stdin from WebSocket to SSH
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		defer stdin.Close()
		log.Debug().Msg("Starting stdin copy")
		n, err := io.Copy(stdin, websocket.NetConn(ctx, conn, websocket.MessageBinary))
		log.Debug().Int64("bytes", n).Err(err).Msg("Stdin copy finished")
	}()

	// Stream stdout from SSH to WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		log.Debug().Msg("Starting stdout copy")
		n, err := io.Copy(websocket.NetConn(ctx, conn, websocket.MessageBinary), stdout)
		log.Debug().Int64("bytes", n).Err(err).Msg("Stdout copy finished")
	}()

	// Stream stderr from SSH to WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		log.Debug().Msg("Starting stderr copy")
		n, err := io.Copy(websocket.NetConn(ctx, conn, websocket.MessageBinary), stderr)
		log.Debug().Int64("bytes", n).Err(err).Msg("Stderr copy finished")
	}()

	wg.Wait()

	log.Debug().Msg("Terminal session ended")

	return nil
}
