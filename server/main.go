package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options

func proxy(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	dest := r.URL.Query().Get("dest")
	if dest == "" {
		log.Println("Empty dest")
		return
	}

	destConn, err := net.DialTimeout("tcp", dest, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()

	transfer(c, destConn)
}

func transfer(ws *websocket.Conn, d io.ReadWriteCloser) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, 100)
		for {
			n, err := d.Read(buf)
			if err == io.EOF {
				msg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "Dest has closed connection")
				ws.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second*3))
				return
			} else if err != nil {
				log.Println("Can not read from dest:", err)
				msg := websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Can not read from dest")
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
			d.Write(message)
		}
	}()

	wg.Wait()
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/proxy", proxy)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
