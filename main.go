package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
)

var (
	addr = ":25571"
)

func init() {
	flag.StringVar(&addr, "addr", addr, "address to listen on")
}

type Messager interface {
	Receive() (int64, []byte, error)
	Send(to int64, msg []byte) error
}

type connection struct {
	id     int64
	conn   net.Conn
	out    chan []byte
	server *server
}

type message struct {
	from int64
	data []byte
}

type server struct {
	addr        string
	nextID      int64
	connections map[int64]*connection
	lock        sync.Locker
	stopper     chan struct{}
	listener    net.Listener
	incoming    chan message
}

type Client struct {
	addr     string
	messages chan []byte
}

func NewClient(serverAddr string) (c *Client, err error) {
	c = &Client{
		addr: serverAddr,
	}

	return
}

func NewServer(addr string) (s *server, err error) {
	s = &server{
		addr:        addr,
		nextID:      1,
		connections: make(map[int64]*connection),
		lock:        &sync.Mutex{},
		stopper:     make(chan struct{}),
	}

	s.listener, err = net.Listen("udp", s.addr)

	if err == nil {
		return
	}

	go s.accepter()
	go s.processor()

	return
}

func (c *connection) reader() {
	buf := make([]byte, 65507)
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			log.Printf("error reading from socket %v", err)
			break
		}

		c.server.incoming <- message{c.id, buf[0:n]}
	}
	// close the out channel?
}

func (c *connection) writer() {
	for b := range c.out {
		n, err := c.conn.Write(b)
		if err != nil {
			log.Printf("error writing: %v", err)
			break
		} else if n != len(b) {
			log.Printf("didn't write enough bytes, expected: %d, written %d", len(b), n)
			break
		}
	}
	// close the connection?
}

func (s *server) processor() {

}

func (s *server) accepter() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// do something besides log here
			log.Printf("accept error %v", err)
			break
		}

		s.lock.Lock()
		c := &connection{
			id:     s.nextID,
			conn:   conn,
			out:    make(chan []byte),
			server: s,
		}
		s.connections[s.nextID] = c
		s.nextID++
		s.lock.Unlock()

		go c.reader()
		go c.writer()
	}
}

func (s *server) Close() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, c := range s.connections {
		c.conn.Close()
		close(c.out)
	}
	s.listener.Close()
}

func main() {
	flag.Parse()

	interupt := make(chan os.Signal)

	signal.Notify(interupt, os.Interrupt)

	s, err := NewServer(addr)
	if err != nil {
		log.Fatal(err)
		return
	}
	<-interupt
	s.Close()
}
