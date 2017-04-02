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
)

type ConnectionManager struct {
	connections []*Connection
	nextConnectionId IdType
	Opened chan ConnectionStateChange
	Closed chan ConnectionStateChange
}

type ConnectionStateChange struct {
	c *Connection
	ack chan bool
}

func (m *ConnectionManager) OpenedConnectionListener() func() {
	return func() {
		log.Println("Opened Connection Listener Started")
		defer log.Println("Opened Connection Listener Stopped")
		for {
			e := <-m.Opened
			log.Println("Got Connection")
			i := m.nextConnectionId
			m.nextConnectionId++
			e.c.Id = i
			m.connections = append(m.connections, e.c)
			log.Printf("Connection Count: %d\n", len(m.connections))
			e.ack <- true
		}
	}
}


func (m *ConnectionManager) ClosedConnectionListener() func() {
	return func() {
		log.Println("Closed Connection Listener Started")
		defer log.Println("Closed Connection Listener Stopped")
	}
}
