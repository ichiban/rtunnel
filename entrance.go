package rtunnel

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	log "github.com/sirupsen/logrus"
)

type Entrance struct {
	Session *yamux.Session
}

func (e *Entrance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodConnect:
		e.outbound(w, r)
	case http.MethodGet:
		e.inbound(w, r) // TODO: authorize exit
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}

func (e *Entrance) outbound(w http.ResponseWriter, r *http.Request) {
	if e.Session == nil {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	log.WithFields(log.Fields{
		"client": r.RemoteAddr,
		"exit":   e.Session.RemoteAddr(),
		"server": r.RequestURI,
	}).Info("start tunneling")
	defer log.WithFields(log.Fields{
		"client": r.RemoteAddr,
		"exit":   e.Session.RemoteAddr(),
		"server": r.RequestURI,
	}).Info("end tunneling")

	in, err := e.Session.OpenStream()
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to open stream")
		e.Session = nil
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer func() { _ = in.Close() }()

	out, err := hijack(w)
	if err != nil {
		log.WithFields(log.Fields{}).Error("failed to hijack")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer func() { _ = out.Close() }()

	r.Header.Set("Client", r.RemoteAddr)
	if err := r.Write(in); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to write")
		return
	}

	Bind(in, out)
}

type hijacked struct {
	net.Conn
	*bufio.ReadWriter
}

func hijack(w http.ResponseWriter) (*hijacked, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response writer is not a hijacker")
	}

	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, fmt.Errorf("failed to hijack: %w", err)
	}

	return &hijacked{
		Conn:       conn,
		ReadWriter: rw,
	}, nil
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (e *Entrance) inbound(w http.ResponseWriter, r *http.Request) {
	log.WithFields(log.Fields{
		"exit": r.RemoteAddr,
	}).Info("connected")
	defer log.WithFields(log.Fields{
		"exit": r.RemoteAddr,
	}).Info("disconnected")

	if e.Session != nil {
		http.Error(w, "", http.StatusConflict)
		return
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to upgrade")
		return
	}

	conn := Conn{base: c}
	s, err := yamux.Client(&conn, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to create client")
		return
	}

	e.Session = s

	<-s.CloseChan()
	e.Session = nil
}

// https://tools.ietf.org/html/rfc7230#section-3.2.6
func quote(a string) string {
	if strings.ContainsAny(a, `(),/:;<=>?@[\]{}`) {
		return fmt.Sprintf(`"%s"`, a)
	}
	return a
}
