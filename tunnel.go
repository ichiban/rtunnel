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

type Tunnel struct {
	Session *yamux.Session
}

func (t *Tunnel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodConnect:
		t.outbound(w, r)
	case http.MethodGet:
		t.inbound(w, r) // TODO: authorize agent
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
}

func (t *Tunnel) outbound(w http.ResponseWriter, r *http.Request) {
	if t.Session == nil {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	log.WithFields(log.Fields{
		"src":   r.RemoteAddr,
		"agent": t.Session.RemoteAddr(),
		"dest":  r.RequestURI,
	}).Info("start tunneling")
	defer log.WithFields(log.Fields{
		"src":   r.RemoteAddr,
		"agent": t.Session.RemoteAddr(),
		"dest":  r.RequestURI,
	}).Info("end tunneling")

	in, err := t.Session.OpenStream()
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to open stream")
		t.Session = nil
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

	r.Header.Add("Forwarded", fmt.Sprintf(`for=%s`, quote(r.RemoteAddr)))
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

func (t *Tunnel) inbound(w http.ResponseWriter, r *http.Request) {
	log.WithFields(log.Fields{
		"agent": r.RemoteAddr,
	}).Info("connected")
	defer log.WithFields(log.Fields{
		"agent": r.RemoteAddr,
	}).Info("disconnected")

	if t.Session != nil {
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

	t.Session = s

	<-s.CloseChan()
	t.Session = nil
}

// https://tools.ietf.org/html/rfc7230#section-3.2.6
func quote(a string) string {
	if strings.ContainsAny(a, `(),/:;<=>?@[\]{}`) {
		return fmt.Sprintf(`"%s"`, a)
	}
	return a
}
