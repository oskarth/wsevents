package wsevents

import (
	"code.google.com/p/go.net/websocket"
	"container/list"
	"encoding/json"
	"fmt"
)

type hub struct {
	connections map[*Connection]bool
	broadcast   chan event
	register    chan *Connection
	unregister  chan *Connection
	connectFunc func(*Connection) // stores connect handler
}

type event struct {
	Name string      `json:"eventName"`
	Data interface{} `json:"data"` // takes arbitrary datas
}

type Connection struct {
	ws       *websocket.Conn
	send     chan event
	eventMap map[string]*list.List
}

var h = hub{
	broadcast:   make(chan event),
	register:    make(chan *Connection),
	unregister:  make(chan *Connection),
	connections: make(map[*Connection]bool),
}

// hub methods

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			fmt.Print("Register: ", h.connections)
			h.connections[c] = true
			if h.connectFunc != nil {
				h.connectFunc(c)
			}
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

// connection methods

func (c *Connection) reader() {
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

func (c *Connection) writer() {
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

// adds callback to an internal event map
func (c *Connection) On(ev string, callback func(interface{})) {
	if c.eventMap[ev] == nil {
		c.eventMap[ev] = new(list.List)
	}
	c.eventMap[ev].PushBack(callback)
}

// calls event (with args if applicable)
func (c *Connection) Emit(ev string, args ...interface{}) {
	handlerList := c.eventMap[ev]
	if handlerList == nil {
		return
	}

	// apply args onto each callback
	elem := handlerList.Front()
	for elem != nil {
		f := elem.Value.(func(...interface{}))
		f(args...)
		elem = elem.Next()
	}
}

func Handler(ws *websocket.Conn) {
	fmt.Print("New websocket connection ", ws)
	c := &Connection{
		send:     make(chan event, 256),
		ws:       ws,
		eventMap: map[string]*list.List{},
	}
	h.register <- c
	defer func() { h.unregister <- c }()
	go c.writer()
	c.reader()
}

func Run() {
	h.run()
}

func Connect(handler func(*Connection)) {
	h.connectFunc = handler
}
