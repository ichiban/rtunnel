package rtunnel

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"

	"github.com/hashicorp/yamux"

	log "github.com/sirupsen/logrus"
)

type Agent struct {
	Tunnel string
}

var dialer = websocket.Dialer{}

func (a *Agent) Run() {
	u, err := url.Parse(a.Tunnel)
	if err != nil {
		log.WithFields(log.Fields{
			"url": a.Tunnel,
		}).Error("malformed tunnel URL")
		return
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		log.WithFields(log.Fields{
			"url": u,
		}).Error("unknown scheme")
	}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		log.WithFields(log.Fields{
			"tunnel": a.Tunnel,
			"err":    err,
		}).Error("failed to connect to tunnel")
		return
	}
	defer func() { _ = conn.Close() }()

	log.WithFields(log.Fields{
		"tunnel": a.Tunnel,
	}).Info("connected")
	defer log.WithFields(log.Fields{
		"tunnel": a.Tunnel,
	}).Info("disconnected")

	c := Conn{base: conn}
	s, err := yamux.Server(&c, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"tunnel": a.Tunnel,
			"err":    err,
		}).Error("failed to create logical server")
		return
	}
	defer func() { _ = s.Close() }()

	for {
		st, err := s.AcceptStream()
		if err != nil {
			if err != io.EOF {
				log.WithFields(log.Fields{
					"tunnel": a.Tunnel,
					"err":    err,
				}).Error("failed to accept stream")
			}
			return
		}

		go func() {
			if err := a.handleStream(st); err != nil {
				log.WithFields(log.Fields{
					"tunnel": a.Tunnel,
					"err":    err,
				}).Error("failed to accept stream")
			}
		}()
	}
}

func (a *Agent) handleStream(out *yamux.Stream) error {
	dest, err := a.destination(out)
	if err != nil {
		return err
	}

	in, err := net.Dial("tcp", dest)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"dest": dest,
	}).Info("start tunneling")
	defer log.WithFields(log.Fields{
		"dest": dest,
	}).Info("end tunneling")

	Bind(in, out)

	return nil
}

func (a *Agent) destination(out *yamux.Stream) (string, error) {
	buf := bufio.NewReader(out)
	req, err := http.ReadRequest(buf)
	if err != nil {
		log.WithFields(log.Fields{
			"tunnel": a.Tunnel,
			"err":    err,
		}).Error("failed to read request")
		return "", err
	}

	resp := http.Response{
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
		StatusCode: http.StatusOK,
	}
	if err := resp.Write(out); err != nil {
		log.WithFields(log.Fields{
			"tunnel": a.Tunnel,
			"err":    err,
		}).Error("failed to write response")
		return "", err
	}

	return req.RequestURI, nil
}
