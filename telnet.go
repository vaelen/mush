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
	"io"
	"log"
)

type TelnetInterceptor struct {
	i io.Reader
	o io.Writer
	Debug bool
}

const (
	T_SE   byte = 240
	T_NOP  byte = 241
	T_Data byte = 242
	T_BRK  byte = 243
	T_IP   byte = 244
	T_AYT  byte = 245
	T_EC   byte = 247
	T_EL   byte = 248
	T_GA   byte = 249
	T_SB   byte = 250
	T_WILL byte = 251
	T_WONT byte = 252
	T_DO   byte = 253
	T_DONT byte = 254
	T_IAC  byte = 255
)

func (t TelnetInterceptor) Read(p []byte) (n int, err error) {
	buf := make([]byte, len(p), cap(p))
	n, err = t.i.Read(buf)
	if err != nil {
		return n, err
	}
	inSeq := false
	var option byte
	var setting byte
	p = p[0:0]
	for i, b := range buf {
		if i >= n {
			break
		}

		if option != 0 && setting != 0 {
			option = 0
			setting = 0
		}

		if inSeq {
			// Look for end of sequence
			switch {
			case option != 0:
				// Third byte of three byte sequence
				if t.Debug {
					log.Printf("Third (Final) Byte: %d\n", b)
				}
				setting = b
			case b == T_IAC:
				// Exit sequence, output character 255
				if t.Debug {
					log.Printf("Escape (Final) Byte: %d\n", b)
				}
				inSeq = false
				option = 0
			case b >= T_SB:
				// Second byte of three byte sequence
				if t.Debug {
					log.Printf("Second Byte: %d\n", b)
				}
				option = b
				continue
			case b >= T_SE:
				// Exit sequence
				if t.Debug {
					log.Printf("Second (Final) Byte: %d\n", b)
				}
				inSeq = false
				continue
			}
		}

		if option != 0 && setting != 0 {
			// Handle settings
			inSeq = false
			continue
		}

		if !inSeq {
			if b == T_IAC {
				inSeq = true
				if t.Debug {
					log.Printf("First Byte: %d\n", b)
				}
				continue
			}
			p = append(p, b)
		}
	}
	return len(p), nil
}
