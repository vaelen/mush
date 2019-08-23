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

/*

import (
	"fmt"
	"github.com/glycerine/zygomys/repl"
)

func (c *Connection) NewLisp() (*zygo.Glisp, *zygo.GlispConfig) {
	glisp := zygo.NewGlispSandbox()
	glisp.StandardSetup()

	// First argument: object, Second argument: text
	glisp.AddFunction("speak", func(env *zygo.Glisp, name string, args []zygo.Sexp) (zygo.Sexp, error) {
		switch v1 := args[0].(type) {
		case *zygo.SexpString:
			o := v1.Val
			switch v2 := args[0].(type) {
			case *zygo.SexpString:
				r := v2.Val

				fmt.Println(r)
			default:
				return nil, fmt.Errorf("second argument must be a string")
			}
		default:
			return nil, fmt.Errorf("first argument must be a string")
		}

		return &zygo.SexpBool{Val: true}, nil
	})

	cfg := zygo.NewGlispConfig("main")

	return glisp, cfg

	//zygo.Repl(glisp, cfg)
}
*/
