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
	"errors"
	"fmt"
	"github.com/abiosoft/ishell"
	"gopkg.in/readline.v1"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

const VersionName = "Vaelen/MUSH Server"
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
	Id            IdType
	C             net.Conn
	Player        *Player
	Shell         *ishell.Shell
	Server        *Server
	Authenticated bool
	Connected     time.Time
	LastActed     time.Time
}

type Server struct {
	cm       *ConnectionManager
	World    *World
	Shutdown chan bool
}

func NewServer() Server {
	cm := &ConnectionManager{
		nextConnectionId: 1,
		connections:      make([]*Connection, 0),
		Opened:           make(chan ConnectionStateChange),
		Closed:           make(chan ConnectionStateChange),
		Shutdown:         make(chan bool),
	}
	go cm.ConnectionManagerThread()()
	w, err := LoadWorld()
	if err != nil {
		log.Fatal(err)
	}
	go w.WorldThread()()
	return Server{
		cm:       cm,
		World:    w,
		Shutdown: make(chan bool),
	}
}

func (s *Server) StartServer(addr string) {
	log.Printf("Starting %s on %s\n", VersionString(), addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		select {
		case <-s.Shutdown:
			log.Printf("Shutting down server\n")
			if s.World != nil {
				ack := make(chan error)
				s.World.SaveWorldState <- SaveWorldStateMessage{Ack: ack}
				<-ack
				s.World.Shutdown <- true
			}
			if s.cm != nil {
				s.cm.Shutdown <- true
			}
			return
		default:
			// Wait for a connection.
			tcpL, ok := l.(*net.TCPListener)
			if ok {
				tcpL.SetDeadline(time.Now().Add(1e9))
			}
			conn, err := l.Accept()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					// Timed out waiting for a connection
					continue
				}
				// This is a real error
				log.Fatal(err)
			}
			go connectionWorker(s.NewConnection(conn))
		}
	}
}

func (s *Server) NewConnection(conn net.Conn) *Connection {
	c := &Connection{
		C: conn,
		Player: &Player{
			Name: "[UNKNOWN]",
		},
		Server:    s,
		Connected: time.Now(),
		LastActed: time.Now(),
	}
	ack := make(chan bool)
	s.cm.Opened <- ConnectionStateChange{c: c, ack: ack}
	<-ack
	return c
}

func (s *Server) Connections() []*Connection {
	return s.cm.Connections()
}

func (c *Connection) String() string {
	r := c.C.RemoteAddr()
	playerName := ""
	if c.Player != nil {
		playerName = c.Player.String()
	}
	s := fmt.Sprintf("[ %d : %s / %s (%s) ]", c.Id, r.Network(), r.String(), playerName)
	return s
}

func (c *Connection) Log(format string, a ...interface{}) {
	log.Printf("%s | %s\n", c.String(), fmt.Sprintf(format, a...))
}

func (c *Connection) Printf(format string, a ...interface{}) {
	if c.Shell != nil {
		c.Shell.Printf(format, a...)
	}
}

func (c *Connection) Println(a ...interface{}) {
	if c.Shell != nil {
		c.Shell.Println(a...)
	}
}

func (c *Connection) ReadLine() string {
	if c.Shell != nil {
		return c.Shell.ReadLine()
	} else {
		return ""
	}
}

func (c *Connection) Close() {
	defer c.C.Close()
	c.Log("Connection closed")
	if c.Authenticated && c.Player != nil {
		c.Server.Wall("%s disapears in a puff of smoke.\n", c.Player.Name)
	}
	ack := make(chan bool)
	c.Server.cm.Closed <- ConnectionStateChange{c: c, ack: ack}
	<-ack
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
		Prompt:              "> ",
		Stdin:               TelnetInterceptor{i: c.C, o: c.C},
		Stdout:              c.C,
		Stderr:              c.C,
		ForceUseInteractive: true,
		UniqueEditLine:      true,
		FuncIsTerminal:      func() bool { return true },
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

	ack := make(chan []*Player)
	c.Server.World.FindPlayer <- FindPlayerMessage{Name: playerName, Ack: ack}
	players := <-ack
	log.Println(players)
	isNew := (len(players) == 0)
	log.Println(len(players))
	var p *Player
	if isNew {
		log.Println("New Player")
		// New Player
		ack := make(chan *Player)
		c.Server.World.NewPlayer <- NewPlayerMessage{
			Name: playerName,
			Ack:  ack,
		}
		p = <-ack
	} else {
		p = players[0]
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
	if p == nil {
		return false, errors.New("Player Not Found")
	}
	c.Player = p
	c.Authenticated = true
	c.Log("Logged In Successfully")
	return isNew, nil
}

// Helper Methods

func (c *Connection) LocationName(loc Location) string {
	locName := "[UNKNOWN]"
	switch loc.Type {
	case L_ROOM:
		r := c.FindRoomById(loc.Id)
		if r != nil {
			locName = r.String()
		}
	case L_PLAYER:
		p := c.FindPlayerById(loc.Id)
		if p != nil {
			locName = p.String()
		}
	case L_ITEM:
		i := c.FindItemById(loc.Id)
		if i != nil {
			locName = i.String()
		}
	}
	return locName
}

func (c *Connection) FindPlayerById(id IdType) *Player {
	ack := make(chan []*Player)
	c.Server.World.FindPlayer <- FindPlayerMessage{Id: id, Ack: ack}
	players := <-ack
	if len(players) == 0 {
		return nil
	} else {
		return players[0]
	}
}

func (c *Connection) FindPlayerByName(name string) *Player {
	ack := make(chan []*Player)
	c.Server.World.FindPlayer <- FindPlayerMessage{Name: name, Ack: ack}
	players := <-ack
	if len(players) == 0 {
		return nil
	} else {
		return players[0]
	}
}

func (c *Connection) NewRoom(name string) *Room {
	if c == nil || !c.Authenticated || c.Player == nil {
		return nil
	}
	ack := make(chan *Room)
	c.Server.World.NewRoom <- NewRoomMessage{Name: name, Owner: c.Player.Id, Ack: ack}
	return <-ack
}

func (c *Connection) FindRoomById(id IdType) *Room {
	ack := make(chan []*Room)
	c.Server.World.FindRoom <- FindRoomMessage{Id: id, Ack: ack}
	rooms := <-ack
	if len(rooms) == 0 {
		return nil
	} else {
		return rooms[0]
	}
}

func (c *Connection) FindRoomsByOwner(id IdType) []*Room {
	ack := make(chan []*Room)
	c.Server.World.FindRoom <- FindRoomMessage{Owner: id, Ack: ack}
	return <-ack
}

func (c *Connection) DestroyRoom(id IdType) *Room {
	if c == nil || !c.Authenticated || c.Player == nil || id < 2 {
		return nil
	}
	r := c.FindRoomById(id)
	if r == nil || (r.Owner != c.Player.Id && !c.Player.Admin) {
		// Can't Destroy
		return nil
	}

	ack := make(chan bool)
	c.Server.World.DestroyRoom <- DestroyRoomMessage{Id: id, Ack: ack}
	<-ack

	return r
}

func (c *Connection) NewItem(name string) *Item {
	if c == nil || !c.Authenticated || c.Player == nil {
		return nil
	}
	ack := make(chan *Item)
	c.Server.World.NewItem <- NewItemMessage{Name: name, Owner: c.Player.Id, Ack: ack}
	return <-ack
}

func (c *Connection) FindItemById(id IdType) *Item {
	ack := make(chan []*Item)
	c.Server.World.FindItem <- FindItemMessage{Id: id, Ack: ack}
	items := <-ack
	if len(items) == 0 {
		return nil
	} else {
		return items[0]
	}
}

func (c *Connection) FindItemsByOwner(id IdType) []*Item {
	ack := make(chan []*Item)
	c.Server.World.FindItem <- FindItemMessage{Owner: id, Ack: ack}
	return <-ack
}

func (c *Connection) FindItemsByLocation(loc Location) []*Item {
	ack := make(chan []*Item)
	c.Server.World.FindItem <- FindItemMessage{Location: &loc, Ack: ack}
	return <-ack
}

func (c *Connection) FindLocalItem(loc Location, nameOrId string) (*Item, []*Item) {
	if c.Player == nil {
		return nil, nil
	}
	var item *Item
	var foundItems []*Item
	n := strings.TrimSpace(strings.ToLower(nameOrId))
	id, err := ParseId(n)
	if err != nil && id > 0 {
		// Look up by id
		i := c.FindItemById(id)
		if i != nil && i.Location == loc {
			item = i
		}
	} else {
		// Look up by name
		items := c.FindItemsByLocation(loc)
		foundItems := make([]*Item, 0)
		for _, i := range items {
			x := strings.TrimSpace(strings.ToLower(i.Name))
			// if this item's name contains the name we were given
			if strings.Contains(x, n) {
				foundItems = append(foundItems, i)
			}
		}
		if len(foundItems) == 1 {
			item = foundItems[0]
			foundItems = nil
		}
	}
	return item, foundItems
}

func (c *Connection) DestroyItem(id IdType) *Item {
	if c == nil || !c.Authenticated || c.Player == nil {
		return nil
	}
	i := c.FindItemById(id)
	if i == nil || (i.Owner != c.Player.Id && !c.Player.Admin) {
		// Can't Destroy
		return nil
	}

	ack := make(chan bool)
	c.Server.World.DestroyItem <- DestroyItemMessage{Id: id, Ack: ack}
	<-ack

	return i
}
