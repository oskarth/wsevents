package wsevents

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
)

type hub struct {
	connections map[*connection]bool
	broadcast   chan event
	register    chan *connection
	unregister  chan *connection
}

type connection struct {
	ws   *websocket.Conn
	send chan event
}

type event struct {
	Name string      `json:"eventName"`
	Data interface{} `json:"data"` // takes arbitrary datas
}

var h = hub{
	broadcast:   make(chan event),
	register:    make(chan *connection),
	unregister:  make(chan *connection),
	connections: make(map[*connection]bool),
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			fmt.Print("Register: ", h.connections)
			h.connections[c] = true
		case c := <-h.unregister:
			delete(h.connections, c)
			close(c.send)
		case ev := <-h.broadcast:
			fmt.Println("Broadcast: ", ev.Name, " : ", ev.Data)
			for c := range h.connections {
				select {
				case c.send <- ev:
				default:
					delete(h.connections, c)
					close(c.send)
					go c.ws.Close()
				}
			}
		}
	}
}

func (c *connection) reader() {
	for {
		var message string
		err := websocket.Message.Receive(c.ws, &message)
		if err != nil {
			break
		}

		var ev event
		if err := json.Unmarshal([]byte(message), &ev); err != nil {
			fmt.Println(err)
			break
		}
		h.broadcast <- ev
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for ev := range c.send {
		msg, err := json.Marshal(ev)
		if err != nil {
			break
		}

		if err := websocket.Message.Send(c.ws, string(msg)); err != nil {
			break
		}
	}
	c.ws.Close()
}

func wsHandler(ws *websocket.Conn) {
	fmt.Print("New websocket connection ", ws)
	c := &connection{send: make(chan event, 256), ws: ws}
	h.register <- c
	defer func() { h.unregister <- c }()
	go c.writer()
	c.reader()
}
