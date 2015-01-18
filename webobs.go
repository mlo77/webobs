package webobs

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"io"
	"net/http"
	"sync"
)

type Message struct {
	tag  string
	data []byte
}

type Client struct {
	readCh  chan Message
	writeCh chan []byte
	ws      *websocket.Conn
	tag     string
	id      int
}

type Server struct {
	clients   map[string][]*Client
	clientRCh chan Message
	writeCh   chan Message
	listeners map[string][]Listener
	mutex     *sync.Mutex

}

type Listener func(tag string, data []byte)

func (s *Server) SetChannel(tag string, l Listener, path string) {
	s.setHandler(tag, path)
	if l != nil {
		s.mutex.Lock()
		if _, ok := s.listeners[tag]; !ok {
			s.listeners[tag] = make([]Listener, 0)
		}
		s.listeners[tag] = append(s.listeners[tag], l)
		s.mutex.Unlock()
	}
}

func (s *Server) addNewClient(tag string, ws *websocket.Conn) *Client {

	s.mutex.Lock()
	if _, ok := s.clients[tag]; !ok {
		s.clients[tag] = make([]*Client, 0)
	}
	s.mutex.Unlock()

	cid := len(s.clients[tag])
	wc := make(chan []byte)
	c := Client{readCh: s.clientRCh, writeCh: wc, ws: ws, tag: tag, id: cid}
	s.mutex.Lock()
	s.clients[tag] = append(s.clients[tag], &c)
	s.mutex.Unlock()
	return &c
}

func (s *Server) removeClient(tag string, id int) {
	s.mutex.Lock()
	if _, ok := s.clients[tag]; !ok {
		return
	}
	s.mutex.Unlock()
	var idx int = -1
	for pos, c := range s.clients[tag] {
		if c.id == id {
			idx = pos
			break
		}
	}
	if idx == -1 {
		return
	}
	s.mutex.Lock()
	cl := s.clients[tag]
	s.clients[tag] = append(cl[:idx], cl[idx+1:]...)
	s.mutex.Unlock()
}

func (s *Server) listen() {
	// listen to clients
	go func() {
		for {
			select {
			case cmsg := <-s.clientRCh:
				fmt.Println("got message from client", cmsg)
				// check listener
				if _, ok := s.listeners[cmsg.tag]; ok {
					for _, listener := range s.listeners[cmsg.tag] {
						listener(cmsg.tag, cmsg.data)
					}
				}
			}
		}
	}()

	// listen to app
	func() {
		for {
			select {
			case amsg := <-s.writeCh:
				fmt.Println("got message from app", amsg)
				if _, ok := s.clients[amsg.tag]; ok {
					for _, cch := range s.clients[amsg.tag] {
						cch.writeCh <- amsg.data
					}
				}
			}
		}
	}()
}

func registerWS(tag string, ws *websocket.Conn, s *Server) {
	fmt.Println("register", tag)

	c := s.addNewClient(tag, ws)
	defer func() {
		s.removeClient(tag, c.id)
	}()

	go c.listenWrite()
	c.listenRead()
}

func (c *Client) listenRead() {
	for {
		select {
		default:
			var data []byte
			err := websocket.Message.Receive(c.ws, &data)
			if err == io.EOF {
				// close client
				return
			} else if err != nil {

			} else {
				c.readCh <- Message{tag: c.tag, data: data}
			}
		}
	}
}

func (c *Client) listenWrite() {
	for {
		select {
		case amsg := <-c.writeCh:
			err := websocket.Message.Send(c.ws, string(amsg))
			if err != nil {
				fmt.Println("send to client pb", amsg, err)
			}
		}
	}
}

func (s *Server) setHandler(tag string, scriptpath string) {
	tagws := tag + "_ws"
	clientscriptpath := "/" + tag + "_res/"

	http.Handle(
		clientscriptpath,
		http.StripPrefix(
			clientscriptpath,
			http.FileServer(http.Dir(scriptpath))))

	http.Handle("/"+tagws, websocket.Handler(
		func(ws *websocket.Conn) {
			registerWS(tag, ws, s)
		}))

	http.HandleFunc("/"+tag, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<h1>%s</h1>"+
			"<script>var soc = new WebSocket(\"ws://\"+window.location.host+\"/%s\");</script>"+
			"<script type=\"text/javascript\" src=\"%s%s\"></script><body></body>",
			tag, tagws, clientscriptpath, tag+".js")
	})
}

func newServer() *Server {
	cs := make(map[string][]*Client)
	crc := make(chan Message)
	wc := make(chan Message)
	l := make(map[string][]Listener)
	mx := &sync.Mutex{}
	return &Server{clients: cs, clientRCh: crc, writeCh: wc, listeners: l, mutex: mx}
}

func StartServer(port string) *Server {
	s := newServer()

	go func() {
		err := http.ListenAndServe(port, nil)
		if err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	}()

	go s.listen()

	return s
}
