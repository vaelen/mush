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

type IdType uint64

type Player struct {
	Id IdType
	Name string
	Room *Room
	Admin bool
}

type World struct {
	playerId IdType
	roomId IdType
	DefaultRoom *Room
	Rooms map[IdType]*Room
	Players map[IdType]*Player
	NewPlayer chan NewPlayerMessage
	NewRoom chan NewRoomMessage
	SaveWorldState chan SaveWorldStateMessage
	Shutdown chan bool
}

func NewWorld() *World {
	w := &World {
		playerId: 1,
		roomId: 1,
		Rooms: make(map[IdType]*Room),
		Players: make(map[IdType]*Player),
		NewPlayer: make(chan NewPlayerMessage),
		NewRoom: make(chan NewRoomMessage),
		SaveWorldState: make(chan SaveWorldStateMessage),
		Shutdown: make(chan bool),
	}
	
	i := w.roomId
	w.roomId++
	w.DefaultRoom = &Room {
		Id: i,
		Name: "Main Lobby",
		Desc: "This is the main lobby.",
	}
	w.Rooms[w.DefaultRoom.Id] = w.DefaultRoom
		
	return w
}

type NewPlayerMessage struct {
	Name string
	Ack chan *Player
}

type NewRoomMessage struct {
	Name string
	Ack chan *Room
}

type SaveWorldStateMessage struct {
	Ack chan error
}

func (w *World) WorldThread() func() {
	return func() {
		log.Println("World Thread Started")
		defer log.Println("World Thread Stopped")
		saveTimer := time.NewTicker(time.Minute * 1).C
		for {
			select {
			case e := <-w.NewPlayer:
				log.Printf("New Player: %s\n", e.Name)
				i := w.playerId
				w.playerId++
				p := &Player{
					Id: i,
					Name: e.Name,
					Room: w.DefaultRoom,
				}
				if p.Id == 1 {
					p.Admin = true
				}
				w.Players[p.Id] = p
				e.Ack <- p
			case e := <-w.NewRoom:
				log.Printf("New Room: %s\n", e.Name)
				i := w.roomId
				w.roomId++
				r := &Room{
					Id: i,
					Name: e.Name,
				}
				w.Rooms[r.Id] = r
				e.Ack <- r
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

func (w *World) FindPlayerByName(name string) *Player {
	n := strings.ToLower(name)
	for _, p := range w.Players {
		pn := strings.ToLower(p.Name)
		if pn == n {
			return p
		}
	}
	return nil
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

type Room struct {
	Id IdType
	Name string
	Desc string
	Exits []Exit
}

type Exit struct {
	Name string
	Desc string
	Dest Room
}
