// Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package crypto

type AESSealer struct{}

func (*AESSealer) Seal(message, key string) (encoded string, err error) {
	enckey := [32]byte{}
	copy(enckey[:], key)
	return AESEncrypt(message, &enckey)
}

func (*AESSealer) Unseal(message, key string) (decoded string, err error) {
	enckey := [32]byte{}
	copy(enckey[:], key)
	return AESDecrypt(message, &enckey)
}
