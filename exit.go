package rtunnel

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"

	"github.com/gorilla/websocket"

	"github.com/hashicorp/yamux"

	log "github.com/sirupsen/logrus"
)

type Exit struct {
	Entrance string
}

var dialer = websocket.Dialer{}

func (e *Exit) Run() {
	u, err := url.Parse(e.Entrance)
	if err != nil {
		log.WithFields(log.Fields{
			"entrance": e.Entrance,
		}).Error("malformed entrance URL")
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
			"entrance": e.Entrance,
			"err":      err,
		}).Error("failed to connect to entrance")
		return
	}
	defer func() { _ = conn.Close() }()

	log.WithFields(log.Fields{
		"entrance": e.Entrance,
	}).Info("connected")
	defer log.WithFields(log.Fields{
		"entrance": e.Entrance,
	}).Info("disconnected")

	c := Conn{base: conn}
	s, err := yamux.Server(&c, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"entrance": e.Entrance,
			"err":      err,
		}).Error("failed to create logical server")
		return
	}
	defer func() { _ = s.Close() }()

	for {
		st, err := s.AcceptStream()
		if err != nil {
			if err != io.EOF {
				log.WithFields(log.Fields{
					"entrance": e.Entrance,
					"err":      err,
				}).Error("failed to accept stream")
			}
			return
		}

		go func() {
			if err := e.handleStream(st); err != nil {
				log.WithFields(log.Fields{
					"entrance": e.Entrance,
					"err":      err,
				}).Error("failed to accept stream")
			}
		}()
	}
}

func (e *Exit) handleStream(out *yamux.Stream) error {
	s, c, err := e.serverAndClient(out)
	if err != nil {
		return err
	}

	in, err := net.Dial("tcp", s)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"client":   c,
		"entrance": e.Entrance,
		"server":   s,
	}).Info("start tunneling")
	defer log.WithFields(log.Fields{
		"client":   c,
		"entrance": e.Entrance,
		"server":   s,
	}).Info("end tunneling")

	Bind(in, out)

	return nil
}

var forwardedPattern = regexp.MustCompile(`\Afor=(|"")"?(.+)"?\z`)

func (e *Exit) serverAndClient(out *yamux.Stream) (string, string, error) {
	buf := bufio.NewReader(out)
	req, err := http.ReadRequest(buf)
	if err != nil {
		log.WithFields(log.Fields{
			"entrance": e.Entrance,
			"err":      err,
		}).Error("failed to read request")
		return "", "", err
	}

	resp := http.Response{
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
		StatusCode: http.StatusOK,
	}
	if err := resp.Write(out); err != nil {
		log.WithFields(log.Fields{
			"entrance": e.Entrance,
			"err":      err,
		}).Error("failed to write response")
		return "", "", err
	}

	return req.RequestURI, req.Header.Get("Client"), nil
}
