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
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// SaveStateFrequency represents how often the game's state should be saved.
const SaveStateFrequency time.Duration = time.Hour

// IDType is the type used for all ID values
type IDType uint64

func (id IDType) String() string {
	return fmt.Sprintf("@%d", id)
}

// ParseID parses a string to an IDType
func ParseID(s string) (IDType, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '@' {
		return 0, fmt.Errorf("ID not formatted properly: %s", s)
	}
	i, err := strconv.ParseUint(s[1:], 10, 64)
	if err != nil {
		return 0, errors.New("Couldn't parse ID: " + s)
	}
	return IDType(i), nil
}

// Player represents a player in the world.
type Player struct {
	ID          IDType
	Name        string
	Description string
	Location    Location
	Admin       bool
	LastActed   time.Time
}

func (p *Player) String() string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%s [%s]", p.Name, p.ID)
}

// Room represents a room in the world.
type Room struct {
	ID          IDType
	Name        string
	Description string
	Exits       []Exit
	Owner       IDType
	Attributes  map[string]string
}

func (r *Room) String() string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("%s [%s]", r.Name, r.ID)
}

// Exit represents an exit between two rooms.
type Exit struct {
	ID              IDType
	Name            string
	Description     string
	LongDescription string
	Destination     IDType
	ArriveMessage   string
	LeaveMessage    string
	Owner           IDType
	Hidden          bool
	Lockable        bool
	Locked          bool
	Key             IDType
	Attributes      map[string]string
}

func (e *Exit) String() string {
	return fmt.Sprintf("%s [%s]", e.Name, e.ID)
}

// Item represents an item in the world.
type Item struct {
	ID          IDType
	Name        string
	Description string
	Owner       IDType
	Location    Location
	Attributes  map[string]string
}

func (i *Item) String() string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%s [%s]", i.Name, i.ID)
}

// LocationType is used to represent the type of a Location.
type LocationType uint8

const (
	// LocationRoom means that the location is a room.
	LocationRoom LocationType = iota
	// LocationPlayer means that the location is a player.
	LocationPlayer
	// LocationItem means that the location is an item.
	LocationItem
)

// Location represents the location of a player or item.
type Location struct {
	ID   IDType
	Type LocationType
}

// PasswordHash stores a password hash
type PasswordHash [sha256.Size]byte

// WorldDatabase holds all of the players, rooms, and items in the world.
type WorldDatabase struct {
	// Data
	NextID      IDType
	DefaultRoom IDType
	Players     map[IDType]*Player
	Rooms       map[IDType]*Room
	Items       map[IDType]*Item
	Auth        map[IDType]PasswordHash
}

// World contains a WorldDatabase and all of the channels needed to modify it.
type World struct {
	// Data
	db WorldDatabase

	// Channels

	FindPlayer    chan FindPlayerMessage
	NewPlayer     chan NewPlayerMessage
	DestroyPlayer chan DestroyPlayerMessage

	FindRoom    chan FindRoomMessage
	NewRoom     chan NewRoomMessage
	DestroyRoom chan DestroyRoomMessage

	FindItem    chan FindItemMessage
	NewItem     chan NewItemMessage
	DestroyItem chan DestroyItemMessage

	SaveWorldState chan SaveWorldStateMessage
	Shutdown       chan bool

	CheckPassword chan PasswordMessage
	SetPassword   chan PasswordMessage
}

// NewWorld creates a new World instance
func NewWorld() *World {
	w := &World{
		db: WorldDatabase{
			NextID:      1,
			DefaultRoom: 1,
			Rooms:       make(map[IDType]*Room),
			Players:     make(map[IDType]*Player),
			Items:       make(map[IDType]*Item),
			Auth:        make(map[IDType]PasswordHash),
		},

		FindPlayer:    make(chan FindPlayerMessage),
		NewPlayer:     make(chan NewPlayerMessage),
		DestroyPlayer: make(chan DestroyPlayerMessage),

		FindRoom:    make(chan FindRoomMessage),
		NewRoom:     make(chan NewRoomMessage),
		DestroyRoom: make(chan DestroyRoomMessage),

		FindItem:    make(chan FindItemMessage),
		NewItem:     make(chan NewItemMessage),
		DestroyItem: make(chan DestroyItemMessage),

		SaveWorldState: make(chan SaveWorldStateMessage),
		Shutdown:       make(chan bool),

		CheckPassword: make(chan PasswordMessage),
		SetPassword:   make(chan PasswordMessage),
	}

	r := &Room{
		ID:          w.nextID(),
		Name:        "Main Lobby",
		Description: "This is the main lobby.",
		Attributes:  make(map[string]string),
	}
	w.db.Rooms[r.ID] = r
	w.db.DefaultRoom = r.ID

	r2 := &Room{
		ID:          w.nextID(),
		Name:        "Cellar",
		Description: "You are in a celler underneath the main lobby.\nTorches on the walls provide light.",
		Attributes:  make(map[string]string),
	}

	w.db.Rooms[r2.ID] = r2

	r.Exits = append(r.Exits, Exit{
		ID:              w.nextID(),
		Name:            "down",
		Description:     "Stairs spiral down from here to the cellar.",
		LongDescription: "You can see the flicker of firelight coming up from below.",
		Destination:     r2.ID,
		ArriveMessage:   "%s comes down the stairs from the main lobby..",
		LeaveMessage:    "%s heads down the stairs to the cellar.",
		Attributes:      make(map[string]string),
	})

	r2.Exits = append(r2.Exits, Exit{
		ID:              w.nextID(),
		Name:            "up",
		Description:     "Stairs spiral up from here to the main lobby.",
		LongDescription: "You can see light shining down from above and you hear the sound of people talking.",
		Destination:     r.ID,
		ArriveMessage:   "%s comes up the stairs from the cellar.",
		LeaveMessage:    "%s heads up the stairs to the main lobby.",
		Attributes:      make(map[string]string),
	})

	return w
}

func (w *World) nextID() IDType {
	i := w.db.NextID
	w.db.NextID++
	return i
}

// FindPlayerMessage is sent to FindPlayer to find a set of players.
type FindPlayerMessage struct {
	ID       IDType
	Name     string
	Location *Location
	Ack      chan []*Player
}

// NewPlayerMessage is sent to NewPlayer to create a new player.
type NewPlayerMessage struct {
	Name  string
	Owner IDType
	Ack   chan *Player
}

// DestroyPlayerMessage is sent to DestroyPlayer to destroy a given player.
type DestroyPlayerMessage struct {
	ID  IDType
	Ack chan bool
}

// FindRoomMessage is sent to FindRoom to find a set of rooms.
type FindRoomMessage struct {
	ID    IDType
	Owner IDType
	Ack   chan []*Room
}

// NewRoomMessage is sent to NewRoom to create a new room.
type NewRoomMessage struct {
	Name  string
	Owner IDType
	Ack   chan *Room
}

// DestroyRoomMessage is sent to DestroyRoom to destroy a room.
type DestroyRoomMessage struct {
	ID  IDType
	Ack chan bool
}

// FindItemMessage is sent to FindItem to find a set of items.
type FindItemMessage struct {
	ID       IDType
	Owner    IDType
	Location *Location
	Ack      chan []*Item
}

// NewItemMessage is sent to NewItem to create a new item.
type NewItemMessage struct {
	Name  string
	Owner IDType
	Ack   chan *Item
}

// DestroyItemMessage is sent to DestroyItem to destroy an item.
type DestroyItemMessage struct {
	ID  IDType
	Ack chan bool
}

// SaveWorldStateMessage is sent to SaveWorldState to save the world's current state to disk.
type SaveWorldStateMessage struct {
	Ack chan error
}

// PasswordMessage is sent to CheckPassword to check a password
// and SetPassword to set a password.
type PasswordMessage struct {
	ID       IDType
	Password string
	Ack      chan bool
}

// WorldThread returns a goroutine that handles World events.
func (w *World) WorldThread() func() {
	return func() {
		log.Println("World Thread Started")
		defer log.Println("World Thread Stopped")
		saveTimer := time.NewTicker(SaveStateFrequency).C
		for {
			select {
			case e := <-w.FindPlayer:
				r := make([]*Player, 0)
				if e.ID > 0 {
					p := w.db.Players[e.ID]
					if p != nil {
						r = append(r, p)
					}
				} else if e.Name != "" {
					p := w.findPlayerByName(e.Name)
					if p != nil {
						r = append(r, p)
					}
				} else if e.Location != nil {
					r = w.findPlayerByLocation(*e.Location)
				}
				e.Ack <- r
			case e := <-w.NewPlayer:
				log.Printf("New Player: %s\n", e.Name)
				id := w.nextID()
				p := &Player{
					ID:   id,
					Name: e.Name,
					Location: Location{
						ID:   w.db.DefaultRoom,
						Type: LocationRoom,
					},
				}
				if len(w.db.Players) == 0 {
					p.Admin = true
				}
				w.db.Players[p.ID] = p
				e.Ack <- p
			case e := <-w.DestroyPlayer:
				if e.ID == 1 {
					e.Ack <- false
				}
				log.Printf("Destroy Player: %d\n", e.ID)
				delete(w.db.Players, e.ID)
				e.Ack <- true
			case e := <-w.FindRoom:
				r := make([]*Room, 0)
				if e.ID > 0 {
					v := w.db.Rooms[e.ID]
					if v != nil {
						r = append(r, v)
					}
				} else if e.Owner > 0 {
					r = w.findRoomByOwner(e.Owner)
				}
				e.Ack <- r
			case e := <-w.NewRoom:
				log.Printf("New Room: %s\n", e.Name)
				id := w.nextID()
				r := &Room{
					ID:         id,
					Name:       e.Name,
					Owner:      e.Owner,
					Attributes: make(map[string]string),
				}
				w.db.Rooms[r.ID] = r
				e.Ack <- r
			case e := <-w.DestroyRoom:
				if e.ID == 1 {
					e.Ack <- false
				}
				log.Printf("Destroy Room: %d\n", e.ID)
				delete(w.db.Rooms, e.ID)
				e.Ack <- true
			case e := <-w.FindItem:
				r := make([]*Item, 0)
				if e.ID > 0 {
					i := w.db.Items[e.ID]
					if i != nil {
						r = append(r, i)
					}
				} else if e.Owner > 0 {
					r = w.findItemByOwner(e.Owner)
				} else if e.Location != nil {
					r = w.findItemByLocation(*e.Location)
				}
				e.Ack <- r
			case e := <-w.NewItem:
				log.Printf("New Item: %s\n", e.Name)
				id := w.nextID()
				i := &Item{
					ID:    id,
					Name:  e.Name,
					Owner: e.Owner,
					Location: Location{
						ID:   e.Owner,
						Type: LocationPlayer,
					},
					Attributes: make(map[string]string),
				}
				w.db.Items[i.ID] = i
				e.Ack <- i
			case e := <-w.DestroyItem:
				log.Printf("Destroy Item: %d\n", e.ID)
				delete(w.db.Items, e.ID)
				e.Ack <- true
			case e := <-w.SaveWorldState:
				e.Ack <- w.saveState()
			case <-saveTimer:
				w.saveState()
			case <-w.Shutdown:
				return
			case e := <-w.CheckPassword:
				// log.Printf("CheckPassword - ID: %s, Password: %s\n", e.ID, e.Password)
				h, ok := w.db.Auth[e.ID]
				h2 := hashPassword(e.Password)
				r := false
				if ok {
					// log.Printf("ID: %s, Stored Hash: %v, Hash: %v\n", e.ID, h, h2)
					if h == h2 {
						r = true
					}
				} else {
					log.Printf("ID: %s, Password Not Found\n", e.ID)
				}
				e.Ack <- r
			case e := <-w.SetPassword:
				// log.Printf("SetPassword - ID: %s, Password: %s\n", e.ID, e.Password)
				w.db.Auth[e.ID] = hashPassword(e.Password)
				e.Ack <- true
			}
		}
	}
}

func hashPassword(pw string) PasswordHash {
	return sha256.Sum256([]byte(pw))
}

func (w *World) findPlayerByName(name string) *Player {
	n := strings.ToLower(name)
	for _, p := range w.db.Players {
		pn := strings.ToLower(p.Name)
		if pn == n {
			return p
		}
	}
	return nil
}

func (w *World) findPlayerByLocation(loc Location) []*Player {
	r := make([]*Player, 0)
	for _, p := range w.db.Players {
		if p.Location == loc {
			r = append(r, p)
		}
	}
	return r
}

func (w *World) findRoomByOwner(id IDType) []*Room {
	r := make([]*Room, 0)
	for _, v := range w.db.Rooms {
		if v.Owner == id {
			r = append(r, v)
		}
	}
	return r
}

func (w *World) findItemByLocation(loc Location) []*Item {
	r := make([]*Item, 0)
	for _, i := range w.db.Items {
		if i.Location == loc {
			r = append(r, i)
		}
	}
	return r
}

func (w *World) findItemByOwner(id IDType) []*Item {
	r := make([]*Item, 0)
	for _, i := range w.db.Items {
		if i.Owner == id {
			r = append(r, i)
		}
	}
	return r
}

func (w *World) saveState() error {
	log.Printf("Saving world state\n")
	mainFn := "world.gob"
	now := time.Now()
	ts := now.Format(time.RFC3339)
	ts = strings.Replace(ts, ":", "", -1)
	fn := fmt.Sprintf("world-%s.gob", ts)
	fn = path.Join("backup", fn)
	os.Mkdir("backup", 0700)
	file, err := os.Create(fn)
	if err != nil {
		log.Printf("ERROR: Could not save world state: %s\n", err.Error())
		return err
	}
	defer file.Close()
	enc := gob.NewEncoder(file)
	err = enc.Encode(&w.db)
	if err != nil {
		log.Printf("ERROR: Could not encode world state: %s\n", err.Error())
		return err
	}
	os.Remove(mainFn)
	err = os.Link(fn, mainFn)
	if err != nil {
		log.Printf("WARNING: Could not link %s to %s: %s\n", fn, mainFn, err.Error())
	}
	// log.Printf("State Saved: %+v", w)
	log.Printf("State Saved\n")
	return nil
}

// LoadWorld loads a World from disk.
func LoadWorld() (*World, error) {
	fn := "world.gob"
	file, err := os.Open(fn)
	w := NewWorld()
	if err != nil {
		log.Printf("WARNING: Previous world state does not exist: %s\n", err.Error())
		w := NewWorld()
		return w, nil
	}
	defer file.Close()
	dec := gob.NewDecoder(file)
	err = dec.Decode(&w.db)
	if err != nil {
		log.Printf("ERROR: Could not load world state: %s\n", err.Error())
		return nil, err
	}
	// log.Printf("State Loaded: %+v", w)
	log.Printf("State Loaded\n")
	return w, nil
}
