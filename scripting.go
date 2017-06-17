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
	"fmt"

	anko_core "github.com/mattn/anko/builtins"
	anko_vm "github.com/mattn/anko/vm"

	//anko_encoding_json "github.com/mattn/anko/builtins/encoding/json"
	//anko_errors "github.com/mattn/anko/builtins/errors"
	//anko_flag "github.com/mattn/anko/builtins/flag"
	//anko_fmt "github.com/mattn/anko/builtins/fmt"
	//anko_io "github.com/mattn/anko/builtins/io"
	//anko_io_ioutil "github.com/mattn/anko/builtins/io/ioutil"
	anko_math "github.com/mattn/anko/builtins/math"
	anko_math_big "github.com/mattn/anko/builtins/math/big"
	anko_math_rand "github.com/mattn/anko/builtins/math/rand"
	//anko_net "github.com/mattn/anko/builtins/net"
	//anko_net_http "github.com/mattn/anko/builtins/net/http"
	//anko_net_url "github.com/mattn/anko/builtins/net/url"
	//anko_os "github.com/mattn/anko/builtins/os"
	//anko_os_exec "github.com/mattn/anko/builtins/os/exec"
	//anko_os_signal "github.com/mattn/anko/builtins/os/signal"
	//anko_path "github.com/mattn/anko/builtins/path"
	//anko_path_filepath "github.com/mattn/anko/builtins/path/filepath"
	anko_regexp "github.com/mattn/anko/builtins/regexp"
	//anko_runtime "github.com/mattn/anko/builtins/runtime"
	anko_sort "github.com/mattn/anko/builtins/sort"
	anko_strings "github.com/mattn/anko/builtins/strings"
	anko_time "github.com/mattn/anko/builtins/time"
)

// ScriptingEnv wraps the scripting environment so that it is isolated from the underlying implementation.
type ScriptingEnv struct {
	c  *Connection
	vm *anko_vm.Env
}

// Test tests that the scripting environment is functioning properly.
func (env *ScriptingEnv) Test() error {
	scope := make(map[string]interface{})
	scope["name"] = "Scripting Tester"
	code := "say(\"%s is functioning properly.\\n\", name)"
	return env.Execute(scope, code)
}

// Execute executes the given code in the given scope.
func (env *ScriptingEnv) Execute(scope map[string]interface{}, code string) error {
	vm := env.vm.NewEnv()
	for k, v := range scope {
		vm.Define(k, v)
	}

	vm.Define("player", env.c.Player)

	_, err := vm.Execute(code)
	return err
}

func (c *Connection) newScriptingEnv() *ScriptingEnv {
	vm := anko_vm.NewEnv()

	// Load safe builtin functions
	anko_core.Import(vm)
	anko_math.Import(vm)
	anko_math_big.Import(vm)
	anko_math_rand.Import(vm)
	anko_regexp.Import(vm)
	anko_sort.Import(vm)
	anko_strings.Import(vm)
	anko_time.Import(vm)

	// Redefine functions
	vm.Define("print", c.Print)
	vm.Define("printf", c.Printf)
	vm.Define("println", c.Println)
	vm.Define("sprintf", fmt.Sprintf)
	vm.Define("log", c.Log)

	vm.Define("foo", 1)
	vm.Define("say", func(format string, a ...interface{}) {
		if c != nil && c.Player != nil && c.Authenticated {
			c.LocationPrintf(&c.Player.Location, format, a...)
		}
	})

	return &ScriptingEnv{c: c, vm: vm}
}
