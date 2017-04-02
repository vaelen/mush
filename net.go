/******
This file is part of Vaelen/MUSH.

Copyright 2017, Andrew Young <andrew@vaelen.org>

    Vaelen/MUSH is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

    Vaelen/MUSH is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
along with Foobar.  If not, see <http://www.gnu.org/licenses/>.
******/

package mush

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"github.com/vaelen/ishell"
	"gopkg.in/readline.v1"
)

const VersionName  = "Vaelen/MUSH Server"
const VersionMajor = 0
const VersionMinor = 0
const VersionPatch = 1
const VersionExtra = ""

func VersionString() string {
	s := fmt.Sprintf("%s v%d.%d.%d", VersionName, VersionMajor, VersionMinor, VersionPatch)
	if VersionExtra != "" {
		s = fmt.Sprintf("%s-%s", s, VersionExtra)
	}
	return s
}

const AuthenticationEnabled = false

type Connection struct {
	Id IdType
	C net.Conn
	Player *Player
	Shell *ishell.Shell
	Server *Server
}

type Server struct {
	cm *ConnectionManager
	World *World
}

func NewServer() Server {
	cm := &ConnectionManager {
		nextConnectionId: 1,
		connections: make([]*Connection,0),
		Opened: make(chan ConnectionStateChange),
		Closed: make(chan ConnectionStateChange),
	}
	go cm.OpenedConnectionListener()()
	go cm.ClosedConnectionListener()()
	w := NewWorld()
	go w.NewPlayerListener()()
	go w.NewRoomListener()()
	return Server{ cm: cm, World: w }
}

func (s *Server) StartServer(addr string) {
	log.Printf("Starting %s on %s\n", VersionString(), addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go connectionWorker(s.NewConnection(conn))
	}
}

func (s *Server) NewConnection(conn net.Conn) *Connection {
	c := &Connection {
		C: conn,
		Player: &Player {
			Name: "[UNKNOWN]",
		},
		Server: s,
	}
	ack := make(chan bool)
	s.cm.Opened <- ConnectionStateChange{c: c, ack: ack}
	<-ack
	return c
}

func (s *Server) Connections() []*Connection {
	c := s.cm.connections
	return c
}

func (c *Connection) String() string {
	r := c.C.RemoteAddr()
	s := fmt.Sprintf("[ %d : %s / %s (%s) ]", c.Id, r.Network(), r.String(), c.Player.Name)
	return s
}

func (c *Connection) Log(format string, a ...interface{}) {
	log.Printf("%s | %s\n", c.String(), fmt.Sprintf(format, a...))
}

func (c *Connection) Printf(format string, a ...interface{}) {
	c.Shell.Printf(format, a...)
}

func (c *Connection) ReadLine() (string, error) {
	return c.Shell.ReadLine()
}

func (c *Connection) Close() {
	defer c.C.Close()
	c.Log("Connection closed")
	if c.Player != nil && c.Player.Name != "" {
		c.Server.Wall("%s disapears in a puff of smoke.\n", c.Player.Name)
	}
}

func connectionWorker(c *Connection) {
	defer c.Close()
	c.Log("Connection opened")
	isNew, err := Login(c)
	if err != nil {
		c.Log("Authentication Failure: %s", err.Error())
		return
	}
	createShell(c)
	if isNew {
		c.Printf("Welcome, %s!\n", c.Player.Name)
	} else {
		c.Printf("Welcome Back, %s!\n", c.Player.Name)
	}
	c.Server.Wall("%s has appeared.\n", c.Player.Name)
	c.Shell.ShowPrompt(true)
	c.Shell.SetPrompt(fmt.Sprintf("%s => ", c.Player.Name))
	addCommands(c)
	c.Look()
	c.Shell.Start()
}

func createShell(c *Connection) {
	c.Shell = ishell.NewWithConfig(&readline.Config{
		Prompt: "> ",
		Stdin: TelnetInterceptor{i: c.C, o: c.C},
		Stdout: c.C,
		Stderr: c.C,
		ForceUseInteractive: true,
		UniqueEditLine: true,
		FuncIsTerminal: func() bool { return true },
		FuncMakeRaw: func() error {
			return nil
		},
		FuncExitRaw: func() error {
			return nil
		},
		FuncGetWidth: func() int {
			return 80
		},
	})
}


func (s *Server) Wall(format string, a ...interface{}) {
	for _, c := range s.Connections() {
		c.Printf(format, a...)
	}
}

func DisableEcho(c io.Writer) {
	// ANSI Escape Sequence to Disable Local Echo
	// b := []byte("\x1b[12h")
	// Telnet sequence to disable echo
	b := []byte{0xFF, 0xFB, 0x01}
	WriteBytes(c, b)
}

func EnableEcho(c io.Writer) {
	// ANSI Escape Sequence to Enable Local Echo
	// b := []byte("\x1b[12h")
	// Telnet sequence to enable echo
	b := []byte{0xFF, 0xFC, 0x01}
	WriteBytes(c, b)
}

func WriteBytes(c io.Writer, b []byte) {
	o := bufio.NewWriter(c)
	o.Write(b)
	o.Flush()
}

func Login(c *Connection) (bool, error) {
	r := bufio.NewReader(TelnetInterceptor{i: c.C, o: c.C})
	w := bufio.NewWriter(c.C)
	buf := make([]byte, 0, 4096)

	fmt.Fprintf(w, "Connected to %s\n\n", VersionString())
	
	fmt.Fprintf(w, "Username => ")
	w.Flush()
	
	n, err := r.ReadString('\n')
	if err != nil {
		return false, err
	}
	playerName := strings.TrimSpace(n)

	p := c.Server.World.FindPlayerByName(playerName)
	isNew := p == nil
	if isNew {
		// New Player
		ack := make(chan *Player)
		c.Server.World.NewPlayer <- NewPlayerMessage {
			Name: playerName,
			Ack: ack,
		}
		p = <-ack
	} else {
		if AuthenticationEnabled {
			fmt.Fprintf(w, "Password => ")
			DisableEcho(w)
			w.Flush()
			// Read any pending bytes
			r.Read(buf)
			
			p, err := r.ReadString('\n')
			if err != nil {
				c.Log("ERROR: %s", err.Error())
				return false, err
			}
			p = strings.TrimSpace(p)
			c.Log("Password: %s", p)
			// TODO: Authenticate
			
			EnableEcho(w)
			w.Flush()
			// Read any pending bytes
			r.Read(buf)
		}
	}
	c.Player = p
	c.Log("Logged In Successfully")
	return isNew, nil
}
