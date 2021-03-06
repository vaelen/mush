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
	"testing"
)

type testPairID struct {
	s string
	i IDType
	e bool
}

var idTests = []testPairID{
	{"@1", 1, false},
	{"@87654", 87654, false},
	{"@0", 0, false},
	{"@-1", 0, true},
	{"0", 0, true},
	{"1234", 0, true},
	{"  @1   ", 1, false},
	{"@  123", 0, true},
}

// TestIDType tests the various functions related to the IDType type.
func TestIDType(t *testing.T) {
	for _, x := range idTests {
		id, err := ParseID(x.s)
		if err != nil && !x.e {
			t.Errorf("ParseID(%s) threw an error when it shouldn't have.", x.s)
		} else if err == nil && id != x.i {
			t.Errorf("ParseID(%s) = %d, but we expected %d.", x.s, id, x.i)
		}
	}
}

func TestPasswordHash(t *testing.T) {
	s := "correct horse battery staple"
	correctHash := PasswordHash{
		196, 187, 203, 31, 190, 201, 157, 101, 191, 89, 216, 92, 140, 182, 46, 226, 219, 150, 63, 15, 225, 6, 244, 131, 217, 175, 167, 59, 212, 227, 154, 138,
	}
	h := hashPassword(s)
	if h != correctHash {
		t.Errorf("hashPassword(%s) = %v, but we expected %v.", s, h, correctHash)
	}
}
