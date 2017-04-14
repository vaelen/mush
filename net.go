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

// VersionName is the name of the server.
const VersionName = "Vaelen/MUSH Server"

// VersionMajor is the major release of the server.
const VersionMajor = 0

// VersionMinor is the minor release of the server.
const VersionMinor = 0

// VersionPatch is the patch release of the server.
const VersionPatch = 1

// VersionExtra contains additional version information about the server.
const VersionExtra = ""

// VersionString outputs the server's version string.
func VersionString() string {
	s := fmt.Sprintf("%s v%d.%d.%d", VersionName, VersionMajor, VersionMinor, VersionPatch)
	if VersionExtra != "" {
		s = fmt.Sprintf("%s-%s", s, VersionExtra)
	}
	return s
}

// AuthenticationEnabled determines whether or not authentication is enabled on the server.
const AuthenticationEnabled = false

// Connection represents a connection to the server.
type Connection struct {
	ID            IDType
	C             net.Conn
	Player        *Player
	Shell         *ishell.Shell
	Server        *Server
	Authenticated bool
	Connected     time.Time
	LastActed     time.Time
}

// Server represents a server instance.
type Server struct {
	cm       *ConnectionManager
	World    *World
	Shutdown chan bool
}

// NewServer creates a new Server instance.
func NewServer() Server {
	cm := NewConnectionManager()
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

// StartServer starts the given Server instance, calling all necessary goroutines.
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
			go connectionWorker(s.newConnection(conn))
		}
	}
}

func (s *Server) newConnection(conn net.Conn) *Connection {
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

// Connections returns the list of open connections.
func (s *Server) Connections() []*Connection {
	return s.cm.Connections()
}

func (c *Connection) String() string {
	r := c.C.RemoteAddr()
	playerName := ""
	if c.Player != nil {
		playerName = c.Player.String()
	}
	s := fmt.Sprintf("[ %d : %s / %s (%s) ]", c.ID, r.Network(), r.String(), playerName)
	return s
}

// Log writes a log entry for the given connection.
func (c *Connection) Log(format string, a ...interface{}) {
	log.Printf("%s | %s\n", c.String(), fmt.Sprintf(format, a...))
}

// Printf writes text to the given connection.
func (c *Connection) Printf(format string, a ...interface{}) {
	if c.Shell != nil {
		c.Shell.Printf(format, a...)
	}
}

// Println writes text to the given connection.
func (c *Connection) Println(a ...interface{}) {
	if c.Shell != nil {
		c.Shell.Println(a...)
	}
}

// ReadLine reads a line of input from the given connection.
func (c *Connection) ReadLine() string {
	if c.Shell != nil {
		return c.Shell.ReadLine()
	}
	return ""
}

// Close closes the given connection.
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
	c.Look("")
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

// Wall writes a string to all open connections.
func (s *Server) Wall(format string, a ...interface{}) {
	for _, c := range s.Connections() {
		c.Printf(format, a...)
	}
}

// DisableEcho sends the telnet escape sequence to disable local echo.
func DisableEcho(c io.Writer) {
	// ANSI Escape Sequence to Disable Local Echo
	// b := []byte("\x1b[12h")
	// Telnet sequence to disable echo
	b := []byte{0xFF, 0xFB, 0x01}
	writeBytes(c, b)
}

// EnableEcho sends the telnet escape sequence to enable local echo.
func EnableEcho(c io.Writer) {
	// ANSI Escape Sequence to Enable Local Echo
	// b := []byte("\x1b[12h")
	// Telnet sequence to enable echo
	b := []byte{0xFF, 0xFC, 0x01}
	writeBytes(c, b)
}

func writeBytes(c io.Writer, b []byte) {
	o := bufio.NewWriter(c)
	o.Write(b)
	o.Flush()
}

// Login performs a login on the given connection.
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

// LocationName is a helper method that returns the name of a given location.
func (c *Connection) LocationName(loc Location) string {
	locName := "[UNKNOWN]"
	switch loc.Type {
	case LocationRoom:
		r := c.FindRoomByID(loc.ID)
		if r != nil {
			locName = r.String()
		}
	case LocationPlayer:
		p := c.FindPlayerByID(loc.ID)
		if p != nil {
			locName = p.String()
		}
	case LocationItem:
		i := c.FindItemByID(loc.ID)
		if i != nil {
			locName = i.String()
		}
	}
	return locName
}

// FindPlayerByID is a helper method that returns a player based on their ID.
func (c *Connection) FindPlayerByID(id IDType) *Player {
	ack := make(chan []*Player)
	c.Server.World.FindPlayer <- FindPlayerMessage{ID: id, Ack: ack}
	players := <-ack
	if len(players) == 0 {
		return nil
	}
	return players[0]
}

// FindPlayerByName is a helper method that returns a player based on their name.
func (c *Connection) FindPlayerByName(name string) *Player {
	ack := make(chan []*Player)
	c.Server.World.FindPlayer <- FindPlayerMessage{Name: name, Ack: ack}
	players := <-ack
	if len(players) == 0 {
		return nil
	}
	return players[0]
}

// NewRoom is a helper method for creating a new room.
func (c *Connection) NewRoom(name string, description string) *Room {
	if c == nil || !c.Authenticated || c.Player == nil {
		return nil
	}
	ack := make(chan *Room)
	c.Server.World.NewRoom <- NewRoomMessage{Name: name, Owner: c.Player.ID, Ack: ack}
	r := <-ack
	r.Description = description
	return r
}

// FindRoomByID is a helper method that returns a room based on its ID.
func (c *Connection) FindRoomByID(id IDType) *Room {
	ack := make(chan []*Room)
	c.Server.World.FindRoom <- FindRoomMessage{ID: id, Ack: ack}
	rooms := <-ack
	if len(rooms) == 0 {
		return nil
	}
	return rooms[0]
}

// FindRoomsByOwner is a helper method that returns a slice of rooms that belong to the given player.
func (c *Connection) FindRoomsByOwner(id IDType) []*Room {
	ack := make(chan []*Room)
	c.Server.World.FindRoom <- FindRoomMessage{Owner: id, Ack: ack}
	return <-ack
}

// DestroyRoom is a helper method that destroys a room.
func (c *Connection) DestroyRoom(id IDType) *Room {
	if c == nil || !c.Authenticated || c.Player == nil || id < 2 {
		return nil
	}
	r := c.FindRoomByID(id)
	if !c.CanDestroyRoom(r) {
		// Can't Destroy
		return nil
	}

	ack := make(chan bool)
	c.Server.World.DestroyRoom <- DestroyRoomMessage{ID: id, Ack: ack}
	<-ack

	return r
}

// NewItem is a helper method that creates a new item.
func (c *Connection) NewItem(name string, description string) *Item {
	if c == nil || !c.Authenticated || c.Player == nil {
		return nil
	}
	ack := make(chan *Item)
	c.Server.World.NewItem <- NewItemMessage{Name: name, Owner: c.Player.ID, Ack: ack}
	i := <-ack
	i.Description = description
	return i
}

// FindItemByID is a helper method that finds an item based on its ID.
func (c *Connection) FindItemByID(id IDType) *Item {
	ack := make(chan []*Item)
	c.Server.World.FindItem <- FindItemMessage{ID: id, Ack: ack}
	items := <-ack
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

// FindItemsByOwner is a helper method that finds a slice of items that belong to the given player.
func (c *Connection) FindItemsByOwner(id IDType) []*Item {
	ack := make(chan []*Item)
	c.Server.World.FindItem <- FindItemMessage{Owner: id, Ack: ack}
	return <-ack
}

// FindItemsByLocation is a helper method that finds a slice of items in a given location.
func (c *Connection) FindItemsByLocation(loc Location) []*Item {
	ack := make(chan []*Item)
	c.Server.World.FindItem <- FindItemMessage{Location: &loc, Ack: ack}
	return <-ack
}

// FindLocalItem is a helper method for finding an item in a given location.
func (c *Connection) FindLocalItem(loc Location, nameOrID string) (*Item, []*Item) {
	if c.Player == nil {
		return nil, nil
	}
	var item *Item
	var foundItems []*Item
	n := strings.TrimSpace(strings.ToLower(nameOrID))
	id, err := ParseID(n)
	if err == nil && id > 0 {
		// Look up by id
		i := c.FindItemByID(id)
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

// DestroyItem is a helper method that destroy an item.
func (c *Connection) DestroyItem(id IDType) *Item {
	if c == nil || !c.Authenticated || c.Player == nil {
		return nil
	}
	i := c.FindItemByID(id)
	if !c.CanDestroyItem(i) {
		// Can't Destroy
		return nil
	}

	ack := make(chan bool)
	c.Server.World.DestroyItem <- DestroyItemMessage{ID: id, Ack: ack}
	<-ack

	return i
}

// CanEditItem returns true of the player can edit the field on the object.
func (c *Connection) CanEditItem(i *Item, field string) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || i == nil || (i.Owner != c.Player.ID && !c.Player.Admin) {
		return false
	}
	return true
}

// CanDestroyItem returns true of the player can destroy the item.
func (c *Connection) CanDestroyItem(i *Item) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || i == nil || (i.Owner != c.Player.ID && !c.Player.Admin) {
		return false
	}
	return true
}

// CanEditRoom returns true if the player can edit the field on the room.
func (c *Connection) CanEditRoom(r *Room, field string) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || r == nil || (r.Owner != c.Player.ID && !c.Player.Admin) {
		return false
	}
	return true
}

// CanDestroyRoom returns true if the player can destroy the room.
func (c *Connection) CanDestroyRoom(r *Room) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || r == nil || (r.Owner != c.Player.ID && !c.Player.Admin) {
		return false
	}
	return true
}

// InLocation returns true if the user is logged in and in the given location.
// Passing in a nil Location reference will return true as long as the user is authenticated.
func (c *Connection) InLocation(loc *Location) bool {
	if c == nil || !c.Authenticated || c.Player == nil {
		return false
	}
	if loc == nil || *loc == c.Player.Location {
		return true
	}
	return false
}
