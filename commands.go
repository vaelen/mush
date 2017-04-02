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
        Help: "say something to the everybody else",
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
        Help: "whisper something to the somebody else",
		LongHelp: "whisper name \"message\"",
        Func: func(e *ishell.Context) {
			if len(e.Args) > 1 {
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
        Name: "look",
        Help: "look around",
        Func: func(e *ishell.Context) {
			c.Look()
        },
    })
}

func (c *Connection) Say(target string, phrase string) {
	t := strings.ToLower(target)
	targetName := ""
	var targetId IdType
	targetId = 0
	conns := c.Server.Connections()

	// Find target name
	if target != "" {
		for _, conn := range conns {
			n := strings.ToLower(conn.Player.Name)
			if t == n {
				targetId = conn.Id
				targetName = conn.Player.Name
				break
			}
		}

		if targetName == "" {
			c.Printf("Couldn't find player %s\n", target)
			return
		}
	}

	// Send messages
	for _, conn := range conns {
		switch {
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
	t := strings.ToLower(target)
	targetName := ""
	var targetId IdType
	targetId = 0
	conns := c.Server.Connections()

	// Find target name
	for _, conn := range conns {
		n := strings.ToLower(conn.Player.Name)
		if t == n {
			targetId = conn.Id
			targetName = conn.Player.Name
			break
		}
	}

	if targetName == "" {
		c.Printf("Couldn't find player %s\n", target)
		return
	}

	for _, conn := range conns {
		switch {
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

func (c *Connection) Look() {
	cId := c.Id
	r := c.Player.Room
	rId := r.Id
	playersHere := make([]string,0)
	for _, conn := range c.Server.Connections() {
		p := conn.Player
		if cId != conn.Id && rId == p.Room.Id {
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
