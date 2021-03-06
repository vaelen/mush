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
along with Vaelen/MUSH.  If not, see <http://www.gnu.org/licenses/>.
******/

package mush

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/abiosoft/ishell"
	"github.com/chzyer/readline"
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
//noinspection GoBoolExpressions
func VersionString() string {
	s := fmt.Sprintf("%s v%d.%d.%d", VersionName, VersionMajor, VersionMinor, VersionPatch)
	if VersionExtra != "" {
		s = fmt.Sprintf("%s-%s", s, VersionExtra)
	}
	return s
}

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
	ScriptingEnv  *ScriptingEnv
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

type listener struct {
	tcp *net.TCPListener
	l   net.Listener
}

func (l *listener) Close() {
	l.l.Close()
}

func (s *Server) newTCPListener(addr string) listener {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	r := listener{l: l}
	tcpL, ok := l.(*net.TCPListener)
	if ok {
		r.tcp = tcpL
	}
	return r
}

func (s *Server) newTLSListener(tlsAddr string) listener {
	l, err := net.Listen("tcp", tlsAddr)
	if err != nil {
		log.Fatal(err)
	}
	tlsL := tls.NewListener(l, s.tlsConfig())
	r := listener{l: tlsL}

	tcpL, ok := l.(*net.TCPListener)
	if ok {
		r.tcp = tcpL
	}
	return r
}

// StartServer starts the given Server instance, calling all necessary goroutines.
func (s *Server) StartServer(addr string, tlsAddr string) {
	log.Printf("Starting %s. Regular: %s, TLS: %s\n", VersionString(), addr, tlsAddr)
	listeners := make([]listener, 0)

	listeners = append(listeners, s.newTCPListener(addr))
	listeners = append(listeners, s.newTLSListener(tlsAddr))

	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()

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
			for x, l := range listeners {
				if l.tcp != nil {
					l.tcp.SetDeadline(time.Now().Add(1e9))
				} else {
					log.Printf("Couldn't set deadline for listener %d.\n", x)
				}
				conn, err := l.l.Accept()
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
}

func (s *Server) tlsConfig() *tls.Config {
	cer, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		log.Fatal(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cer}}
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
	c.ScriptingEnv = c.newScriptingEnv()
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
func (c *Connection) Log(s string) {
	c.Logf(s)
}

// Logf writes a log entry for the given connection.
func (c *Connection) Logf(format string, a ...interface{}) {
	log.Printf("%s | %s\n", c.String(), fmt.Sprintf(format, a...))
}

// Printf writes text to the given connection.
func (c *Connection) Printf(format string, a ...interface{}) {
	if c != nil {
		c.Print(fmt.Sprintf(format, a...))
	}
}

// Println writes text to the given connection, followed by a new line character.
func (c *Connection) Println(message string) {
	if c != nil {
		c.Print(message + "\n")
	}
}

// Print writes the text to the given connection without transforming it.
func (c *Connection) Print(a ...interface{}) {
	if c != nil && c.Shell != nil {
		// TODO: Replace this with a channel message.
		c.Shell.Print(a...)
	}
}

// LocationPrintf sends text to all of the players in a given location.
func (c *Connection) LocationPrintf(loc *Location, fmt string, a ...interface{}) {
	if c == nil || c.Shell == nil {
		return
	}
	for _, conn := range c.Server.Connections() {
		if conn.InLocation(loc) {
			conn.Printf(fmt, a...)
		}
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
		c.LocationPrintf(&c.Player.Location, "%s disapears in a puff of smoke.\n", c.Player.Name)
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
		c.Logf("Authentication Failure: %s", err.Error())
		return
	}
	createShell(c)
	if isNew {
		c.Printf("Welcome, %s!\n", c.Player.Name)
	} else {
		c.Printf("Welcome Back, %s!\n", c.Player.Name)
	}
	c.LocationPrintf(&c.Player.Location, "%s has appeared.\n", c.Player.Name)
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

	fmt.Fprintf(w, "Connected to %s\n\n", VersionString())

	fmt.Fprint(w, "Username => ")
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
	isNew := len(players) == 0
	log.Println(len(players))
	var p *Player
	if isNew {
		log.Println("New Player")
		// New Player
		var pw string
		fmt.Fprint(w, "Welcome new player!\n")
		fmt.Fprint(w, "When choosing a password, please don't use one you normally use elsewhere.\n")
		w.Flush()
		for {
			pw, err = readPassword("Choose Password => ", r, w)
			if err != nil {
				return false, err
			}
			fmt.Fprint(w, "\n")
			pv, err := readPassword("Retype Password => ", r, w)
			if err != nil {
				return false, err
			}
			fmt.Fprint(w, "\n")
			if pw == pv {
				break
			} else {
				_, err = fmt.Fprint(w, "Passwords didn't match, please try again.\n")
				if err != nil {
					return false, err
				}
			}
		}
		ack := make(chan *Player)
		c.Server.World.NewPlayer <- NewPlayerMessage{
			Name: playerName,
			Ack:  ack,
		}
		p = <-ack
		c.setPassword(p.ID, pw)
	} else {
		p = players[0]
		i := 0
		for {
			i++
			pw, err := readPassword("Password => ", r, w)
			if err != nil {
				return false, err
			}
			fmt.Fprint(w, "\n")
			if c.checkPassword(p.ID, pw) {
				break
			} else if i >= 3 {
				fmt.Fprint(w, "Authentication failed.\n")
				w.Flush()
				c.C.Close()
				return false, fmt.Errorf("authentication failed: %s", p.Name)
			}
		}
	}
	if p == nil {
		return false, errors.New("player not found")
	}
	c.Player = p
	c.Authenticated = true
	c.Log("Logged In Successfully")
	return isNew, nil
}

func readPassword(prompt string, r *bufio.Reader, w *bufio.Writer) (string, error) {
	buf := make([]byte, 0, 4096)
	fmt.Fprintf(w, prompt)
	DisableEcho(w)
	w.Flush()
	// Read any pending bytes
	r.Read(buf)

	p, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	p = strings.TrimSpace(p)

	EnableEcho(w)
	w.Flush()
	// Read any pending bytes
	r.Read(buf)
	return p, nil
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

// FindAllPlayers returns all players in the database.
// TODO: Replace with a channel
func (c *Connection) FindAllPlayers() []*Player {
	if c == nil {
		return nil
	}
	players := make([]*Player, 0)
	for _, p := range c.Server.World.db.Players {
		players = append(players, p)
	}
	return players
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

// FindOnlinePlayersByLocation is a helper method that returns a slice of players who are online and in the given location.
// Passing in nil will return all players who are currently online.
func (c *Connection) FindOnlinePlayersByLocation(loc *Location) []*Player {
	players := make([]*Player, 0)
	for _, conn := range c.Server.Connections() {
		if conn.InLocation(loc) {
			players = append(players, conn.Player)
		}
	}
	return players
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

// FindAllRooms returns all rooms in the database.
// TODO: Replace with a channel
func (c *Connection) FindAllRooms() []*Room {
	if c == nil {
		return nil
	}
	rooms := make([]*Room, 0)
	for _, r := range c.Server.World.db.Rooms {
		rooms = append(rooms, r)
	}
	return rooms
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

// NewExit is a helper method for creating a new exit.
func (c *Connection) NewExit(name string, description string) *Exit {
	if c == nil || !c.Authenticated || c.Player == nil || c.Player.Location.Type != LocationRoom {
		return nil
	}
	room := c.Player.Location.ID
	r := c.FindRoomByID(room)
	if r == nil || !c.CanEditRoom(r, "Exits") {
		// Can't Destroy
		return nil
	}
	ack := make(chan *Exit)
	c.Server.World.NewExit <- NewExitMessage{Room: room, Name: name, Owner: c.Player.ID, Ack: ack}
	ex := <-ack
	if ex != nil {
		ex.Description = description
	}
	return ex
}

// DestroyExit is a helper method that destroys an exit.
func (c *Connection) DestroyExit(id IDType) *Exit {
	if c == nil || !c.Authenticated || c.Player == nil || c.Player.Location.Type != LocationRoom {
		return nil
	}
	room := c.Player.Location.ID
	r := c.FindRoomByID(room)
	if r == nil || !c.CanEditRoom(r, "Exits") {
		// Can't Destroy
		return nil
	}

	var ex *Exit
	for _, ex = range r.Exits {
		if ex.ID == id {
			break
		}
	}

	if ex == nil {
		return nil
	}

	ack := make(chan bool)
	c.Server.World.DestroyExit <- DestroyExitMessage{Room: room, ID: id, Ack: ack}
	<-ack

	return ex
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

// FindAllItems returns all items in the database.
// TODO: Replace with a channel
func (c *Connection) FindAllItems() []*Item {
	if c == nil {
		return nil
	}
	items := make([]*Item, 0)
	for _, i := range c.Server.World.db.Items {
		items = append(items, i)
	}
	return items
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
	if c == nil || !c.Authenticated || c.Player == nil || i == nil {
		return false
	}
	if i.Owner != c.Player.ID && !c.IsAdmin() {
		return false
	}
	return true
}

// CanDestroyItem returns true of the player can destroy the item.
func (c *Connection) CanDestroyItem(i *Item) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || i == nil {
		return false
	}
	if i.Owner != c.Player.ID && !c.IsAdmin() {
		return false
	}
	return true
}

// CanEditPlayer returns true if the player can edit the field on the room.
func (c *Connection) CanEditPlayer(p *Player, field string) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || p == nil {
		return false
	}
	if p.ID != c.Player.ID && !c.IsAdmin() {
		return false
	}
	return true
}

// CanDestroyPlayer returns true if the player can destroy the room.
func (c *Connection) CanDestroyPlayer(p *Player) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || p == nil {
		return false
	}
	if p.ID != c.Player.ID && !c.IsAdmin() {
		return false
	}
	return true
}

// CanEditRoom returns true if the player can edit the field on the room.
func (c *Connection) CanEditRoom(r *Room, field string) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || r == nil {
		return false
	}
	if r.Owner != c.Player.ID && !c.IsAdmin() {
		return false
	}
	return true
}

// CanDestroyRoom returns true if the player can destroy the room.
func (c *Connection) CanDestroyRoom(r *Room) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || r == nil {
		return false
	}
	if r.Owner != c.Player.ID && !c.IsAdmin() {
		return false
	}
	return true
}

// CanEditExit returns true if the player can edit the field on the room.
func (c *Connection) CanEditExit(e *Exit, field string) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || e == nil {
		return false
	}
	if e.Owner != c.Player.ID && !c.IsAdmin() {
		return false
	}
	return true
}

// CanDestroyExit returns true if the player can destroy the room.
func (c *Connection) CanDestroyExit(e *Exit) bool {
	// TODO: This can be made more granular later.
	if c == nil || !c.Authenticated || c.Player == nil || e == nil {
		return false
	}
	if e.Owner != c.Player.ID && !c.IsAdmin() {
		return false
	}
	return true
}

// FindLocalThing is a helper method for finding an item, player, or exit in a given location.
func (c *Connection) FindLocalThing(loc Location, nameOrID string, includeExits bool) (foundOne fmt.Stringer, foundMany []fmt.Stringer) {
	if c.Player == nil {
		return nil, nil
	}
	n := strings.TrimSpace(strings.ToLower(nameOrID))
	id, err := ParseID(n)
	if err == nil && id > 0 {
		// Look up by id
		i := c.FindItemByID(id)
		if i != nil && (i.Location == loc || c.IsAdmin()) {
			foundOne = i
		}
		if foundOne == nil {
			l := &loc
			if c.IsAdmin() {
				l = nil
			}
			for _, p := range c.FindOnlinePlayersByLocation(l) {
				if p.ID == id {
					foundOne = p
					break
				}
			}
		}
		if foundOne == nil {
			r := c.FindRoomByID(id)
			if r != nil && (loc.ID == id || c.IsAdmin()) {
				foundOne = r
			}
		}
		if foundOne == nil && includeExits && loc.Type == LocationRoom {
			r := c.FindRoomByID(loc.ID)
			if r != nil {
				for _, e := range r.Exits {
					if e.ID == id {
						foundOne = e
						break
					}
				}
			}
		}
	} else {
		// Look up by name
		foundMany = make([]fmt.Stringer, 0)
		things := make([]fmt.Stringer, 0)
		switch loc.Type {
		case LocationItem:
			i := c.FindItemByID(loc.ID)
			if i != nil {
				things = append(things, i)
			}
		case LocationPlayer:
			p := c.FindPlayerByID(loc.ID)
			if p != nil {
				things = append(things, p)
			}
		case LocationRoom:
			r := c.FindRoomByID(loc.ID)
			if r != nil {
				things = append(things, r)
				if includeExits {
					for _, e := range r.Exits {
						things = append(things, e)
					}
				}
			}
		}
		for _, i := range c.FindItemsByLocation(loc) {
			things = append(things, i)
		}
		for _, p := range c.FindOnlinePlayersByLocation(&loc) {
			things = append(things, p)
		}
		for _, t := range things {
			x := strings.TrimSpace(strings.ToLower(t.String()))
			// if this item's name contains the name we were given
			if strings.Contains(x, n) {
				foundMany = append(foundMany, t)
			}
		}
		if len(foundMany) == 1 {
			foundOne = foundMany[0]
			foundMany = nil
		}
	}
	return foundOne, foundMany
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

func (c *Connection) checkPassword(id IDType, pw string) bool {
	ack := make(chan bool)
	c.Server.World.CheckPassword <- PasswordMessage{ID: id, Password: pw, Ack: ack}
	return <-ack
}

func (c *Connection) setPassword(id IDType, pw string) bool {
	ack := make(chan bool)
	c.Server.World.SetPassword <- PasswordMessage{ID: id, Password: pw, Ack: ack}
	return <-ack
}

// ExecuteScriptWithScope executes the given code within the given scope.
func (c *Connection) ExecuteScriptWithScope(scope map[string]interface{}, code string) error {
	if c == nil || c.ScriptingEnv == nil {
		c.Log("Couldn't Execute Script")
		return fmt.Errorf("Couldn't Execute Script")
	}

	return c.ScriptingEnv.Execute(scope, code)
}

// ExecuteScript executes the given code.
func (c *Connection) ExecuteScript(code string) error {
	if c == nil || c.ScriptingEnv == nil {
		c.Log("Couldn't Execute Script")
		return fmt.Errorf("Couldn't Execute Script")
	}
	scope := make(map[string]interface{})
	return c.ScriptingEnv.Execute(scope, code)
}

// TestScriptingEnvironment tests that the scripting environment is functioning properly.
func (c *Connection) TestScriptingEnvironment() error {
	if c == nil || c.ScriptingEnv == nil {
		c.Log("Couldn't Execute Script")
		return fmt.Errorf("Couldn't Execute Script")
	}

	return c.ScriptingEnv.Test()
}
