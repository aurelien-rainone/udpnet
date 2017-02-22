package udp

import (
	"errors"
	"fmt"
	"net"
)

// ConnCallback is the interface implemented by objects that handles the main
// events happening on a connection, start, connect, disconnect and stop.
type ConnCallback interface {
	OnStart()
	OnStop()
	OnConnect()
	OnDisconnect()
}

type ConnMode int

const (
	None ConnMode = iota
	Client
	Server
)

type connState int

const (
	disconnected = iota
	listening
	connecting
	connectFail
	connected
)

type Conn struct {
	protocolId         uint
	timeout            float64
	running            bool
	mode               ConnMode
	state              connState
	socket             Socket
	timeoutAccumulator float64
	address            *net.UDPAddr
	cb                 ConnCallback
}

func NewConn(cb ConnCallback, protocolId uint, timeout float64) *Conn {
	c := &Conn{
		protocolId: protocolId,
		timeout:    timeout,
		mode:       None,
		running:    false,
		cb:         cb,
	}
	c.clearData()
	return c
}

//virtual ~Connection()
//{
//if ( IsRunning() )
//Stop();
//}

func (c *Conn) Start(port int) bool {
	//assert( !running );
	fmt.Printf("start connection on port %d\n", port)
	if err := c.socket.Open(port); err != nil {
		return false
	}
	c.running = true
	c.cb.OnStart()
	return true
}

func (c *Conn) Stop() {
	//assert( running );
	fmt.Printf("stop connection\n")
	connected := c.IsConnected()
	c.clearData()
	c.socket.Close()
	c.running = false
	if connected {
		c.cb.OnDisconnect()
	}
	c.cb.OnStop()
}

func (c *Conn) IsRunning() bool {
	return c.running
}

func (c *Conn) Listen() {
	fmt.Printf("server listening for connection\n")
	connected := c.IsConnected()
	c.clearData()
	if connected {
		c.cb.OnDisconnect()
	}
	c.mode = Server
	c.state = listening
}

func (c *Conn) Connect(address *net.UDPAddr) {
	fmt.Printf("client connecting to %v\n", *address)
	isConnected := c.IsConnected()
	c.clearData()
	if isConnected {
		c.cb.OnDisconnect()
	}
	c.mode = Client
	c.state = connecting
	c.address = address
}

func (c *Conn) IsConnecting() bool {
	return c.state == connecting
}

func (c *Conn) ConnectFailed() bool {
	return c.state == connectFail
}

func (c *Conn) IsConnected() bool {
	return c.state == connected
}

func (c *Conn) IsListening() bool {
	return c.state == listening
}

func (c *Conn) GetMode() ConnMode {
	return c.mode
}

//virtual
func (c *Conn) Update(deltaTime float64) {
	//assert( running );
	c.timeoutAccumulator += deltaTime
	if c.timeoutAccumulator > c.timeout {
		if c.state == connecting {
			fmt.Printf("connect timed out\n")
			c.clearData()
			c.state = connectFail
			c.cb.OnDisconnect()
		} else if c.state == connected {
			fmt.Printf("connection timed out\n")
			c.clearData()
			if c.state == connecting {
				c.state = connectFail
			}
			c.cb.OnDisconnect()
		}
	}
}

//virtual
func (c *Conn) SendPacket(data []byte) error {
	//assert( running );
	if c.address == nil {
		return errors.New("address not set")
	}
	packet := make([]byte, len(data)+4)
	packet[0] = byte(c.protocolId >> 24)
	packet[1] = byte((c.protocolId >> 16) & 0xFF)
	packet[2] = byte((c.protocolId >> 8) & 0xFF)
	packet[3] = byte((c.protocolId) & 0xFF)
	copy(packet[4:], data)
	return c.socket.Send(c.address, packet)
}

//virtual
func (c *Conn) ReceivePacket(data []byte) int {
	//assert( running );
	packet := make([]byte, len(data)+4)
	var sender net.UDPAddr
	bytesRead := c.socket.Receive(&sender, packet)
	if bytesRead == 0 {
		return 0
	}
	if bytesRead <= 4 {
		return 0
	}
	if packet[0] != byte(c.protocolId>>24) ||
		packet[1] != byte((c.protocolId>>16)&0xFF) ||
		packet[2] != byte((c.protocolId>>8)&0xFF) ||
		packet[3] != byte(c.protocolId&0xFF) {
		return 0
	}
	if c.mode == Server && !c.IsConnected() {
		fmt.Printf("server accepts connection from client %v\n",
			sender.String())
		c.state = connected
		c.address = &sender
		c.cb.OnConnect()
	}
	// TODO: Aurelien, should check if this the only way to compare two net.UDPAddr
	if sender.String() == c.address.String() {
		if c.mode == Client && c.state == connecting {
			fmt.Printf("client completes connection with server\n")
			c.state = connected
			c.cb.OnConnect()
		}
		c.timeoutAccumulator = 0.0
		copy(data, packet[4:])
		return len(data) - 4
	}
	return 0
}

func (c *Conn) HeaderSize() int {
	return 4
}

func (c *Conn) clearData() {
	c.state = disconnected
	c.timeoutAccumulator = 0.0
	c.address = nil
}