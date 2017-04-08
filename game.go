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
	"encoding/gob"
	"fmt"
	"os"
	"log"
	"strings"
	"time"
)

const SaveStateFrequency time.Duration = time.Minute * 30

type IdType uint64

type Player struct {
	Id IdType
	Name string
	Location Location
	Admin bool
}

func (p *Player) String() string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%s [P: %d]", p.Name, p.Id)
}

type Room struct {
	Id IdType
	Name string
	Desc string
	Exits []Exit
	Owner IdType
	Attr map[string]string
}

func (r *Room) String() string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("%s [R: %d]", r.Name, r.Id)
}

func NewRoom() *Room {
	return &Room{
		Attr: make(map[string]string),
	}
}

type Exit struct {
	Id IdType
	Name string
	Desc string
	Dest Room
	Owner IdType
	Hidden bool
	Lockable bool
	Locked bool
	Key IdType
	Attr map[string]string
}

func NewExit() *Exit {
	return &Exit{
		Attr: make(map[string]string),
	}
}

type Item struct {
	Id IdType
	Name string
	Desc string
	Owner IdType
	Location Location
	Attr map[string]string
}

func (i *Item) String() string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%s [I: %d]", i.Name, i.Id)
}

func NewItem() *Item {
	return &Item{
		Attr: make(map[string]string),
	}
}


type LocationType uint8

const (
	L_ROOM LocationType = iota
	L_PLAYER
	L_ITEM
)

type Location struct {
	Id IdType
	Type LocationType
}

type World struct {
	// Data
	playerId IdType
	roomId IdType
	itemId IdType
	DefaultRoom IdType
	players map[IdType]*Player
	rooms map[IdType]*Room
	items map[IdType]*Item

	// Channels
	FindPlayer chan FindPlayerMessage
	NewPlayer chan NewPlayerMessage
	DestroyPlayer chan DestroyPlayerMessage
	
	FindRoom chan FindRoomMessage
	NewRoom chan NewRoomMessage
	DestroyRoom chan DestroyRoomMessage
	
	FindItem chan FindItemMessage
	NewItem chan NewItemMessage
	DestroyItem chan DestroyItemMessage
	
	SaveWorldState chan SaveWorldStateMessage
	Shutdown chan bool
}

func NewWorld() *World {
	w := &World {
		playerId: 1,
		roomId: 1,
		itemId: 1,
		DefaultRoom: 1,
		rooms: make(map[IdType]*Room),
		players: make(map[IdType]*Player),
		items: make(map[IdType]*Item),

		FindPlayer: make(chan FindPlayerMessage),
		NewPlayer: make(chan NewPlayerMessage),
		DestroyPlayer: make(chan DestroyPlayerMessage),

		FindRoom: make(chan FindRoomMessage),
		NewRoom: make(chan NewRoomMessage),
		DestroyRoom: make(chan DestroyRoomMessage),

		SaveWorldState: make(chan SaveWorldStateMessage),
		Shutdown: make(chan bool),
	}
	
	i := w.roomId
	w.roomId++
	r := &Room {
		Id: i,
		Name: "Main Lobby",
		Desc: "This is the main lobby.",
	}
	w.rooms[r.Id] = r
	w.DefaultRoom = r.Id
		
	return w
}

type FindPlayerMessage struct {
	Id IdType
	Name string
	Location *Location
	Ack chan []*Player
}

type NewPlayerMessage struct {
	Name string
	Owner IdType
	Ack chan *Player
}

type DestroyPlayerMessage struct {
	Id IdType
	Ack chan bool
}

type FindRoomMessage struct {
	Id IdType
	Owner IdType
	Ack chan []*Room
}

type NewRoomMessage struct {
	Name string
	Owner IdType
	Ack chan *Room
}

type DestroyRoomMessage struct {
	Id IdType
	Ack chan bool
}

type FindItemMessage struct {
	Id IdType
	Owner IdType
	Location *Location
	Ack chan []*Item
}

type NewItemMessage struct {
	Name string
	Owner IdType
	Ack chan *Item
}

type DestroyItemMessage struct {
	Id IdType
	Ack chan bool
}

type SaveWorldStateMessage struct {
	Ack chan error
}

func (w *World) WorldThread() func() {
	return func() {
		log.Println("World Thread Started")
		defer log.Println("World Thread Stopped")
		saveTimer := time.NewTicker(SaveStateFrequency).C
		for {
			select {
			case e:= <-w.FindPlayer:
				r := make([]*Player, 0)
				if e.Id > 0 {
					p := w.players[e.Id]
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
				id := w.playerId
				w.playerId++
				p := &Player{
					Id: id,
					Name: e.Name,
					Location: Location {
						Id: w.DefaultRoom,
						Type: L_ROOM,
					},
				}
				if p.Id == 1 {
					p.Admin = true
				}
				w.players[p.Id] = p
				e.Ack <- p
			case e:= <-w.DestroyPlayer:
				if e.Id == 1 {
					return
				}
				log.Printf("Destroy Player: %s\n", e.Id)
				delete(w.players, e.Id)
			case e:= <-w.FindRoom:
				r := make([]*Room, 0)
				if e.Id > 0 {
					v := w.rooms[e.Id]
					if v != nil {
						r = append(r, v)
					}
				} else if e.Owner > 0 {
					r = w.findRoomByOwner(e.Owner)
				}
				e.Ack <- r
			case e := <-w.NewRoom:
				log.Printf("New Room: %s\n", e.Name)
				id := w.roomId
				w.roomId++
				r := &Room{
					Id: id,
					Name: e.Name,
					Owner: e.Owner,
				}
				w.rooms[r.Id] = r
				e.Ack <- r
			case e:= <-w.DestroyRoom:
				if e.Id == 1 {
					return
				}
				log.Printf("Destroy Room: %s\n", e.Id)
				delete(w.rooms, e.Id)
			case e:= <-w.FindItem:
				r := make([]*Item, 0)
				if e.Id > 0 {
					i := w.items[e.Id]
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
				id := w.itemId
				w.itemId++
				i := &Item{
					Id: id,
					Name: e.Name,
					Owner: e.Owner,
					Location: Location {
						Id: e.Owner,
						Type: L_PLAYER,
					},
				}
				w.items[i.Id] = i
				e.Ack <- i
			case e:= <-w.DestroyPlayer:
				if e.Id == 1 {
					return
				}
				log.Printf("Destroy Player: %s\n", e.Id)
				delete(w.players, e.Id)
			case e:= <-w.SaveWorldState:
				e.Ack <- w.saveState()
			case <-saveTimer:
				w.saveState()
			case <-w.Shutdown:
				return
			}
		}
	}
}

func (w *World) findPlayerByName(name string) *Player {
	n := strings.ToLower(name)
	for _, p := range w.players {
		pn := strings.ToLower(p.Name)
		if pn == n {
			return p
		}
	}
	return nil
}

func (w *World) findPlayerByLocation(loc Location) []*Player {
	r := make([]*Player, 0)
	for _, p := range w.players {
		if p.Location == loc  {
			r = append(r, p)
		}
	}
	return r
}

func (w *World) findRoomByOwner(id IdType) []*Room {
	r := make([]*Room, 0)
	for _, v := range w.rooms {
		if v.Owner == id  {
			r = append(r, v)
		}
	}
	return r
}

func (w *World) findItemByLocation(loc Location) []*Item {
	r := make([]*Item, 0)
	for _, i := range w.items {
		if i.Location == loc  {
			r = append(r, i)
		}
	}
	return r
}

func (w *World) findItemByOwner(id IdType) []*Item {
	r := make([]*Item, 0)
	for _, i := range w.items {
		if i.Owner == id  {
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
	file, err := os.Create(fn)
	if err != nil {
		log.Printf("ERROR: Could not save world state: %s\n", err.Error())
		return err
	}
	defer file.Close()
	enc := gob.NewEncoder(file)
	err = enc.Encode(w)
	if err != nil {
		log.Printf("ERROR: Could not encode world state: %s\n", err.Error())
		return err
	}
	os.Remove(mainFn)
	err = os.Link(fn, mainFn)
	if err != nil {
		log.Printf("WARNING: Could not link %s to %s: %s\n", fn, mainFn, err.Error())
	}
	return nil
}

func LoadWorld() (*World, error) {
	fn := "world.gob"
	w := NewWorld()
	file, err := os.Open(fn)
	if err != nil {
		log.Printf("WARNING: Previous world state does not exist: %s\n", err.Error())
		return w, nil
	}
	defer file.Close()
	dec := gob.NewDecoder(file)
	err = dec.Decode(w)
	if err != nil {
		log.Printf("ERROR: Could not load world state: %s\n", err.Error())
		return nil, err
	}
	return w, nil
}

