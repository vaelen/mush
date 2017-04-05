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
	"fmt"
	"strings"
	"time"
	"github.com/vaelen/ishell"
)

func addCommands(c *Connection) {
	shell := c.Shell
	player := c.Player
	
	shell.AddCmd(&ishell.Cmd{
        Name: "exit",
        Help: "log off",
        Func: func(e *ishell.Context) {
            e.Printf("Goodbye, %s\n", player.Name)
			e.Stop()
        },
    })

	shell.AddCmd(&ishell.Cmd{
        Name: "say",
        Help: "say something to the everybody else. say [player] <message>",
		LongHelp: "say [name] \"message\"",
        Func: func(e *ishell.Context) {
			if len(e.Args) > 0 {
				var target string
				var phrase string
				if len(e.Args) > 1 {
					target = e.Args[0]
					phrase = e.Args[1]
				} else {
					target = ""
					phrase = e.Args[0]
				}
				c.Log("Executing Say: %s - %s", target, phrase)
				c.Say(target, phrase)
			} else {
				c.Printf(e.Cmd.HelpText())
			}
        },
    })
	
	shell.AddCmd(&ishell.Cmd{
        Name: "whisper",
        Help: "whisper something to the somebody else. whisper <player> <message>",
		LongHelp: "whisper name \"message\"",
        Func: func(e *ishell.Context) {
			if len(e.Args) > 1 {
				c.updateIdleTime()
				target := e.Args[0]
				phrase := e.Args[1]
				c.Log("Executing Whisper: %s - %s", target, phrase)
				c.Whisper(target, phrase)
			} else {
				c.Printf(e.Cmd.HelpText())
			}
        },
    })

	shell.AddCmd(&ishell.Cmd{
        Name: "emote",
        Help: "emote something. emote <action>",
		LongHelp: "emote \"action\"",
        Func: func(e *ishell.Context) {
			if len(e.Args) > 0 {
				c.updateIdleTime()
				action := e.Args[0]
				c.Log("Executing Emote: %s", action)
				c.Emote(action)
			} else {
				c.Printf(e.Cmd.HelpText())
			}
        },
    })
	
	shell.AddCmd(&ishell.Cmd{
        Name: "look",
        Help: "look around",
        Func: func(e *ishell.Context) {
			c.updateIdleTime()
			c.Look()
        },
    })

	shell.AddCmd(&ishell.Cmd{
        Name: "who",
        Help: "see who's online",
        Func: func(e *ishell.Context) {
			c.updateIdleTime()
			c.Who()
        },
    })

}

func (c *Connection) updateIdleTime() {
	c.LastActed = time.Now()
}

func (c *Connection) findPlayerConnectionByName(target string) (targetId IdType, targetName string) {
	t := strings.ToLower(target)
	for _, conn := range c.Server.Connections() {
		if conn.Authenticated && conn.Player != nil {
			n := strings.ToLower(conn.Player.Name)
			if t == n {
				targetId = conn.Id
				targetName = conn.Player.Name
				break
			}
		}
	}
	return
}

func (c *Connection) Say(target string, phrase string) {
	targetId, targetName := c.findPlayerConnectionByName(target)
	
	if target != "" {
		if targetName == "" {
			c.Printf("Couldn't find player %s\n", target)
			return
		}
	}

	// Send messages
	for _, conn := range c.Server.Connections() {
		switch {
		case !conn.Authenticated:
			// Do Nothing
		case conn.Id == c.Id:
			// Do Nothing
		case conn.Id == targetId && target != "":
			conn.Printf("%s says \"%s\" to you.\n", c.Player.Name, phrase)			
		case target == "":
			conn.Printf("%s says \"%s\".\n", c.Player.Name, phrase)
		default:
			conn.Printf("%s says \"%s\" to %s.\n", c.Player.Name, phrase, targetName)
		}
	}
	if target == "" {
		c.Printf("You say \"%s\".\n", phrase)
	} else {
		c.Printf("You say \"%s\" to %s.\n", phrase, targetName)
	}

}

func (c *Connection) Whisper(target string, phrase string) {
	targetId, targetName := c.findPlayerConnectionByName(target)

	if targetName == "" {
		c.Printf("Couldn't find player %s\n", target)
		return
	}

	for _, conn := range c.Server.Connections() {
		switch {
		case !conn.Authenticated:
			// Do Nothing
		case conn.Id == c.Id:
			// Do Nothing
		case conn.Id == targetId:
			conn.Printf("%s whispers \"%s\".\n", c.Player.Name, phrase)			
		default:
			conn.Printf("%s whispers to %s.\n", c.Player.Name, targetName)
		}
	}
	c.Printf("You whisper \"%s\" to %s.\n", phrase, targetName)
}

func (c *Connection) Emote(action string) {
	for _, conn := range c.Server.Connections() {
		switch {
		case !conn.Authenticated:
			// Do Nothing
		default:
			conn.Printf("%s %s.\n", c.Player.Name, action)
		}
	}
}

func (c *Connection) Look() {
	cId := c.Id
	r := c.Player.Room
	rId := r.Id
	playersHere := make([]string,0)
	for _, conn := range c.Server.Connections() {
		p := conn.Player
		if conn.Authenticated && p != nil && p.Room != nil && cId != conn.Id && rId == p.Room.Id {
			playersHere = append(playersHere, p.Name)
		}
	}
	s := fmt.Sprintf("[%s (%d)]\n", r.Name, r.Id)
	s += r.Desc + "\n"
	for _, pName := range playersHere {
		s += fmt.Sprintf("You see %s here.\n", pName)
	}
	s += "\n"
	c.Printf(s)
}

// TODO: Have the column widths auto-adjust to fit the data
func (c *Connection) Who() {
	s := "Players Currently Online:\n"
	f := "%10s %20s %20s %30s %15s\n"
	s += fmt.Sprintf(f, "Connection", "Player", "Location", "Connected", "Idle")
	h10 := "----------"
	h15 := "---------------"
	h20 := "--------------------"
	h30 := "------------------------------"
	s += fmt.Sprintf(f, h10, h20, h20, h30, h15)
	for _, conn := range c.Server.Connections() {
		playerName := "[Authenticating]"
		roomName := "[UNKNOWN]"
		var roomId IdType

		if conn.Authenticated && conn.Player != nil {
			playerName = conn.Player.Name
			if conn.Player.Room != nil {
				roomName = conn.Player.Room.Name
				roomId = conn.Player.Room.Id
			}
		}

		connId := fmt.Sprintf("%10d", conn.Id)
		roomString := fmt.Sprintf("%s [%d]", roomName, roomId)
		connected := conn.Connected.Format(time.RFC1123)
		idle := time.Since(conn.LastActed).String()
		
		s += fmt.Sprintf(f, connId, playerName, roomString, connected, idle)

	}
	s += "\n"
	c.Printf(s)
}
