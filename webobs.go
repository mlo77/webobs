// Package webobs provides creation of websocket handlers at runtime,
// allowing easy communication between go program and web browsers
package webobs

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"sync"
)

type Message struct {
	// Tag categories the message
	Tag string
	// Data contains data, may be marshalled to JSON
	Data []byte
}

type clientobs struct {
	readCh  chan Message
	writeCh chan []byte
	ws      *websocket.Conn
	tag     string
	id      int
}

type Server struct {
	clients   map[string][]*clientobs
	clientRCh chan Message
	// WriteCh is used whenever the application wish to send messages
	// the web observers
	WriteCh   chan Message
	listeners map[string][]Listener
	mutex     *sync.Mutex
}

// Listener is the callback type when receiving messages from web observers
type Listener func(tag string, data []byte)

// SetChannel adds a websocket handle and http handle to serve rendered
// html pages and scripts
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

func (s *Server) addNewclientobs(tag string, ws *websocket.Conn) *clientobs {

	s.mutex.Lock()
	if _, ok := s.clients[tag]; !ok {
		s.clients[tag] = make([]*clientobs, 0)
	}
	s.mutex.Unlock()

	cid := len(s.clients[tag])
	wc := make(chan []byte)
	c := clientobs{readCh: s.clientRCh, writeCh: wc, ws: ws, tag: tag, id: cid}
	s.mutex.Lock()
	s.clients[tag] = append(s.clients[tag], &c)
	s.mutex.Unlock()
	return &c
}

func (s *Server) removeclientobs(tag string, id int) {
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
				//fmt.Println("got message from client", cmsg)
				// check listener
				if _, ok := s.listeners[cmsg.Tag]; ok {
					for _, listener := range s.listeners[cmsg.Tag] {
						listener(cmsg.Tag, cmsg.Data)
					}
				}
			}
		}
	}()

	// listen to app
	func() {
		for {
			select {
			case amsg := <-s.WriteCh:
				//fmt.Println("got message from app", amsg)
				if _, ok := s.clients[amsg.Tag]; ok {
					for _, cch := range s.clients[amsg.Tag] {
						cch.writeCh <- amsg.Data
					}
				}
			}
		}
	}()
}

func registerWS(tag string, ws *websocket.Conn, s *Server) {
	fmt.Println("register", tag)

	c := s.addNewclientobs(tag, ws)
	defer func() {
		s.removeclientobs(tag, c.id)
	}()

	go c.listenWrite()
	c.listenRead()
}

func (c *clientobs) listenRead() {
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
				c.readCh <- Message{Tag: c.tag, Data: data}
			}
		}
	}
}

func (c *clientobs) listenWrite() {
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

	tmpl := scriptpath + "/" + tag + ".html"
	if _, err := os.Stat(tmpl); os.IsNotExist(err) {
		http.HandleFunc("/"+tag, func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "<h1>%s</h1>"+
				"<script>var soc = new WebSocket(\"ws://\"+window.location.host+\"/%s\");</script>"+
				"<script type=\"text/javascript\" src=\"%s%s\"></script><body></body>",
				tag, tagws, clientscriptpath, tag+".js")
		})
	} else {
		http.HandleFunc("/"+tag, func(w http.ResponseWriter, r *http.Request) {
			t, _ := template.ParseFiles(tmpl)
			p := struct{ 
				Title string 
				Tagws string
				ScriptPath string
				ScriptName string
			}{
				tag, 
				tagws,
				clientscriptpath,
				tag+".js",
			}
			t.Execute(w, p)
		})
	}
}

func newServer() *Server {
	cs := make(map[string][]*clientobs)
	crc := make(chan Message)
	wc := make(chan Message)
	l := make(map[string][]Listener)
	mx := &sync.Mutex{}
	return &Server{clients: cs, clientRCh: crc, WriteCh: wc, listeners: l, mutex: mx}
}

// StartServer creates and starts a http server with no handles,
// it awaits for channels to be set.
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
