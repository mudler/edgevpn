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

package service

import (
	"strings"

	"github.com/ipfs/go-log"
)

// Role is a service role.
// It is identified by a unique string which is sent over the wire
// and streamed to/from the clients.
// Roles can be applied either directly, or assigned within roles in the API
type Role string

// RoleConfig is the role config structure, which holds all the objects that can be used by a Role
type RoleConfig struct {
	Client                                              *Client
	UUID, ServiceID, StateDir, APIAddress, NetworkToken string
	Logger                                              log.StandardLogger

	roles map[Role]func(c *RoleConfig) error
}

// RoleOption is a role option
type RoleOption func(c *RoleConfig)

// RoleKey is an association between a Role(string) and a Handler which actually
// fullfills the role
type RoleKey struct {
	RoleHandler func(c *RoleConfig) error
	Role        Role
}

// WithRole sets the available roles
func WithRole(f map[Role]func(c *RoleConfig) error) RoleOption {
	return func(c *RoleConfig) {
		c.roles = f
	}
}

// WithRoleLogger sets a logger for the role action
func WithRoleLogger(l log.StandardLogger) RoleOption {
	return func(c *RoleConfig) {
		c.Logger = l
	}
}

// WithRoleUUID sets the UUID which performs the role
func WithRoleUUID(u string) RoleOption {
	return func(c *RoleConfig) {
		c.UUID = u
	}
}

// WithRoleStateDir sets the statedir for the role
func WithRoleStateDir(s string) RoleOption {
	return func(c *RoleConfig) {
		c.StateDir = s
	}
}

// WithRoleToken sets the network token which can be used by the role
func WithRoleToken(s string) RoleOption {
	return func(c *RoleConfig) {
		c.NetworkToken = s
	}
}

// WithRoleAPIAddress sets the API Address used during the execution
func WithRoleAPIAddress(s string) RoleOption {
	return func(c *RoleConfig) {
		c.APIAddress = s
	}
}

// WithRoleServiceID sets a role service ID
func WithRoleServiceID(s string) RoleOption {
	return func(c *RoleConfig) {
		c.ServiceID = s
	}
}

// WithRoleClient sets a client for a role
func WithRoleClient(e *Client) RoleOption {
	return func(c *RoleConfig) {
		c.Client = e
	}
}

// Apply applies a role and takes a list of options
func (rr Role) Apply(opts ...RoleOption) {
	c := &RoleConfig{}
	for _, o := range opts {
		o(c)
	}

	for _, role := range strings.Split(string(rr), ",") {
		r := Role(role)
		if f, exists := c.roles[r]; exists {
			c.Logger.Info("Role loaded. Applying ", r)
			if err := f(c); err != nil {
				c.Logger.Warning("Failed applying role", role, err)
			}
		} else {
			c.Logger.Warn("Unknown role: ", r)
		}
	}
}
