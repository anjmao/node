/*
 * Copyright (C) 2017 The "MysteriumNetwork/node" Authors.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package version

import (
	"fmt"

	"gopkg.in/urfave/cli.v1"
)

// NewCommand function creates version command
func NewCommand(versionSummary string) *cli.Command {
	return &cli.Command{
		Name:      "version",
		Usage:     "Show version",
		ArgsUsage: " ",
		Action: func(ctx *cli.Context) error {
			_, err := fmt.Fprintln(ctx.App.Writer, versionSummary)
			return err
		},
	}
}
