package vm

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	messageWait    = 10 * time.Second
	maxMessageSize = 1024
)

type WinSize struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  maxMessageSize,
	WriteBufferSize: maxMessageSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Service) Terminal(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("failed to init websocket")
		return
	}
	defer func() {
		_ = ws.Close()
	}()

	cmd := exec.Command("/bin/login")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		log.Errorf("failed to start pty")
		return
	}
	defer func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Signal(os.Interrupt)
		}
		if err := cmd.Wait(); err != nil {
			log.Debug("terminal process exited")
		}
	}()

	go wsWrite(ws, ptmx)
	wsRead(ws, ptmx)
}

// pty to ws
func wsWrite(ws *websocket.Conn, ptmx *os.File) {
	data := make([]byte, maxMessageSize)

	for {
		n, err := ptmx.Read(data)
		if err != nil {
			return
		}

		if n > 0 {
			_ = ws.SetWriteDeadline(time.Now().Add(messageWait))

			err = ws.WriteMessage(websocket.BinaryMessage, data[:n])
			if err != nil {
				log.Errorf("write ws message failed: %s", err)
				return
			}
		}
	}
}

// ws to pty
func wsRead(ws *websocket.Conn, ptmx *os.File) {
	_ = ws.SetReadDeadline(time.Now().Add(messageWait))

	for {
		msgType, p, err := ws.ReadMessage()
		if err != nil {
			return
		}

		switch msgType {
		case websocket.BinaryMessage:
			var winSize WinSize
			if err := json.Unmarshal(p, &winSize); err == nil {
				rows := clampTerminalDimension(winSize.Rows, 1, 200)
				cols := clampTerminalDimension(winSize.Cols, 1, 300)
				_ = pty.Setsize(ptmx, &pty.Winsize{
					Rows: rows,
					Cols: cols,
				})
			}
			continue
		}

		if _, writeErr := ptmx.Write(p); writeErr != nil {
			log.Errorf("failed to write to pty")
			return
		}
	}
}

func clampTerminalDimension(value uint16, min uint16, max uint16) uint16 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
