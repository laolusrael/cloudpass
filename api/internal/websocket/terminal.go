package websocket

import (
	"context"
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
	client multipass.Client
}

func NewTerminalHandler(client multipass.Client) *TerminalHandler {
	return &TerminalHandler{client: client}
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

	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()

	sshClient := multipass.NewSSHClient(30)
	if err := sshClient.Connect(ip, 30); err != nil {
		log.Error().Err(err).Str("ip", ip).Msg("failed to SSH connect")
		wsjson.Write(ctx, conn, map[string]string{
			"type": "error",
			"data": fmt.Sprintf("failed to connect via SSH: %v", err),
		})
		return nil
	}
	defer sshClient.Close()

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

	// Handle WebSocket messages (resize events)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		for {
			var msg TerminalMessage
			err := wsjson.Read(ctx, conn, &msg)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Debug().Err(err).Msg("websocket read error")
				return
			}

			if msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
				err := session.WindowChange(msg.Rows, msg.Cols)
				if err != nil {
					log.Warn().Err(err).Msg("failed to resize terminal")
				}
			}
		}
	}()

	// Stream stdin from WebSocket to SSH
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		defer stdin.Close()
		io.Copy(stdin, websocket.NetConn(ctx, conn, websocket.MessageBinary))
	}()

	// Stream stdout from SSH to WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		io.Copy(websocket.NetConn(ctx, conn, websocket.MessageBinary), stdout)
	}()

	// Stream stderr from SSH to WebSocket
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		io.Copy(websocket.NetConn(ctx, conn, websocket.MessageBinary), stderr)
	}()

	wg.Wait()

	return nil
}
