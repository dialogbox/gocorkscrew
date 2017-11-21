package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")
var encrypted = flag.Bool("tls", true, "use tls")

func handleTunneling(w http.ResponseWriter, r *http.Request) {
	scheme := "wss"
	if !*encrypted {
		scheme = "ws"
	}
	u := url.URL{Scheme: scheme, Host: *addr, Path: "/proxy", RawQuery: fmt.Sprintf("dest=%s", r.Host)}
	log.Printf("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer c.Close()

	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Hijacking not supported")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}

	transfer(c, clientConn)
}

func transfer(ws *websocket.Conn, c io.ReadWriteCloser) {
	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, 100)
		for {
			n, err := c.Read(buf)
			if err == io.EOF {
				msg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "Client has closed connection")
				ws.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second*3))
				return
			} else if err != nil {
				log.Println("Can not read from client:", err)
				msg := websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Can not read from client")
				ws.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second*3))
				return
			}
			ws.WriteMessage(websocket.BinaryMessage, buf[:n])
		}
	}()

	go func() {
		defer wg.Done()
		for {
			_, message, err := ws.ReadMessage()
			if websocket.IsCloseError(err, websocket.CloseGoingAway) {
				log.Println("Websocket is closed")
				return
			}
			if err != nil {
				log.Println("Can not read from websocket:", err)
				return
			}
			c.Write(message)
		}
	}()

	wg.Wait()
	log.Println("Done")
}

func handleHTTP(w http.ResponseWriter, req *http.Request) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func main() {
	var pemPath string
	flag.StringVar(&pemPath, "pem", "server.pem", "path to pem file")
	var keyPath string
	flag.StringVar(&keyPath, "key", "server.key", "path to key file")
	var proto string
	flag.StringVar(&proto, "proto", "http", "Proxy protocol (http or https)")
	flag.Parse()
	if proto != "http" && proto != "https" {
		log.Fatal("Protocol must be either http or https")
	}
	server := &http.Server{
		Addr: "localhost:8888",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleTunneling(w, r)
			} else {
				handleHTTP(w, r)
			}
		}),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	if proto == "http" {
		log.Fatal(server.ListenAndServe())
	} else {
		log.Fatal(server.ListenAndServeTLS(pemPath, keyPath))
	}
}
