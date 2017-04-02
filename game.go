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
	"log"
	"strings"
)

type IdType uint64

type Player struct {
	Id IdType
	Name string
	Room *Room
}

type World struct {
	playerId IdType
	roomId IdType
	DefaultRoom *Room
	Rooms map[IdType]*Room
	Players map[IdType]*Player
	NewPlayer chan NewPlayerMessage
	NewRoom chan NewRoomMessage
}

func NewWorld() *World {
	w := &World {
		playerId: 1,
		roomId: 1,
		Rooms: make(map[IdType]*Room),
		Players: make(map[IdType]*Player),
		NewPlayer: make(chan NewPlayerMessage),
		NewRoom: make(chan NewRoomMessage),
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

func (w *World) NewPlayerListener() func() {
	return func() {
		log.Println("New Player Listener Started")
		defer log.Println("New Player Listener Stopped")
		for {
			e := <-w.NewPlayer
			log.Printf("New Player: %s\n", e.Name)
			i := w.playerId
			w.playerId++
			p := &Player{
				Id: i,
				Name: e.Name,
				Room: w.DefaultRoom,
			}
			w.Players[p.Id] = p
			e.Ack <- p
		}
	}
}

type NewRoomMessage struct {
	Name string
	Ack chan *Room
}

func (w *World) NewRoomListener() func() {
	return func() {
		log.Println("New Room Listener Started")
		defer log.Println("New Room Listener Stopped")
		for {
			e := <-w.NewRoom
			log.Printf("New Room: %s\n", e.Name)
			i := w.roomId
			w.roomId++
			r := &Room{
				Id: i,
				Name: e.Name,
			}
			w.Rooms[r.Id] = r
			e.Ack <- r
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
