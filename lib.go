package wsevents

import (
	"code.google.com/p/go.net/websocket"
	"container/list"
	"encoding/json"
	"fmt"
)

type hub struct {
	connections map[*Connection]bool
	register    chan *Connection
	unregister  chan *Connection
	connectFunc func(*Connection) // stores connect handler
	rooms       map[string]*list.List
}

type event struct {
	Name string      `json:"eventName"`
	Data interface{} `json:"data"` // takes arbitrary datas
}

// label n candidate?
type iceCandidate struct {
	Label     string `json:"label"`
	Candidate string `json:"candidate"`
}

type Connection struct {
	ws       *websocket.Conn
	send     chan event
	eventMap map[string]*list.List
}

var h = hub{
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
			c.init()
			if h.connectFunc != nil {
				h.connectFunc(c)
			}
		case c := <-h.unregister:
			delete(h.connections, c)
			close(c.send)
		}
	}
}

// connection methods

func (c *Connection) init() {
	c.On("join_room", func(room string) {
		// generate random Id
		if h.rooms[room] == nil {
			h.rooms[room] = new(list.List)
		}
		h.rooms[room].PushBack(c)
		// broadcast the new peer to a certain room
	})

	c.On("send_ice_candidate", func(ice iceCandidate) {
		data := map[string]string{
			"label":     ice.label,
			"candidate": ice.candidate,
			"socketId":  c.Id,
		}
		c.Emit("receive_ice_candidate", data)
	})

	c.On("send_offer", func(msg interface{}) {
		// where?
	})

	c.On("send_answer", func(msg interface{}) {
		// where?
	})
}

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

		// get the handler list associated with an event
		handlerList := c.eventMap[ev.Name]
		if handlerList == nil {
			fmt.Println("No associated events")
			continue
		}

		// apply args onto each callback
		elem := handlerList.Front()
		for elem != nil {
			f := elem.Value.(func(interface{}))
			f(ev.Data)
			elem = elem.Next()
		}
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
func (c *Connection) Emit(eventName string, data interface{}) {
	var ev event
	ev.Name = eventName
	ev.Data = data
	c.send <- ev
}

// TODO: need to support broadcasting "to" a room.
func (c *Connection) Broadcast(ev string, data interface{}) {
	for conn := range h.connections {
		if conn != c {
			conn.Emit(ev, data)
		}
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
