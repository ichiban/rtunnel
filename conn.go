package rtunnel

import (
	"bytes"
	"io"
	"net"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type Conn struct {
	base *websocket.Conn
	buf  bytes.Buffer
}

func (c *Conn) Read(p []byte) (n int, err error) {
	if c.buf.Len() == 0 {
		messageType, p, err := c.base.ReadMessage()
		if err != nil {
			if err, ok := err.(*websocket.CloseError); ok {
				switch err.Code {
				case websocket.CloseNormalClosure, websocket.CloseAbnormalClosure:
					return 0, io.EOF
				}
			}

			log.WithFields(log.Fields{
				"err": err,
			}).Error("failed to read message")
			return 0, err
		}
		if messageType == websocket.BinaryMessage {
			c.buf.Write(p)
		}
	}

	return c.buf.Read(p)
}

func (c *Conn) Write(p []byte) (n int, err error) {
	return len(p), c.base.WriteMessage(websocket.BinaryMessage, p)
}

func (c *Conn) Close() error {
	return c.base.Close()
}

func (c *Conn) LocalAddr() net.Addr {
	return c.base.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.base.RemoteAddr()
}
