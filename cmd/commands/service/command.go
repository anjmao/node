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

package service

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mysteriumnetwork/node/cmd"
	"github.com/mysteriumnetwork/node/cmd/commands/license"
	"github.com/mysteriumnetwork/node/config/urfavecli/clicontext"
	"github.com/mysteriumnetwork/node/core/service"
	"github.com/mysteriumnetwork/node/identity"
	identity_selector "github.com/mysteriumnetwork/node/identity/selector"
	"github.com/mysteriumnetwork/node/logconfig"
	"github.com/mysteriumnetwork/node/metadata"
	openvpn_service "github.com/mysteriumnetwork/node/services/openvpn/service"
	"github.com/mysteriumnetwork/node/services/shared"
	wireguard_service "github.com/mysteriumnetwork/node/services/wireguard/service"
	"github.com/mysteriumnetwork/node/tequilapi/client"
	"github.com/pkg/errors"
	"gopkg.in/urfave/cli.v1"
	"gopkg.in/urfave/cli.v1/altsrc"
)

var log = logconfig.NewLogger()

var (
	identityFlag = altsrc.NewStringFlag(cli.StringFlag{
		Name:  "identity",
		Usage: "Keystore's identity used to provide service. If not given identity will be created automatically",
		Value: "",
	})
	identityPassphraseFlag = altsrc.NewStringFlag(cli.StringFlag{
		Name:  "identity.passphrase",
		Usage: "Used to unlock keystore's identity",
		Value: "",
	})
	agreedTermsConditionsFlag = altsrc.NewBoolFlag(cli.BoolFlag{
		Name:  "agreed-terms-and-conditions",
		Usage: "Agree with terms & conditions",
	})
)

// NewCommand function creates service command
func NewCommand(licenseCommandName string) *cli.Command {
	var di cmd.Dependencies
	command := &cli.Command{
		Name:      "service",
		Usage:     "Starts and publishes services on Mysterium Network",
		ArgsUsage: "comma separated list of services to start",
		Before:    clicontext.LoadUserConfigQuietly,
		Action: func(ctx *cli.Context) error {
			if !ctx.Bool(agreedTermsConditionsFlag.Name) {
				printTermWarning(licenseCommandName)
				os.Exit(2)
			}

			quit := make(chan error)
			nodeOptions := cmd.ParseFlagsNode(ctx)
			if err := di.Bootstrap(nodeOptions); err != nil {
				return err
			}
			go func() { quit <- di.Node.Wait() }()

			cmd.RegisterSignalCallback(func() { quit <- nil })

			shared.Configure(ctx)
			cmdService := &serviceCommand{
				tequilapi:    client.NewClient(nodeOptions.TequilapiAddress, nodeOptions.TequilapiPort),
				errorChannel: quit,
				ap: client.AccessPoliciesRequest{
					IDs: shared.ConfiguredOptions().AccessPolicies,
				},
			}

			go func() {
				quit <- cmdService.Run(ctx)
			}()

			return describeQuit(<-quit)
		},
		After: func(ctx *cli.Context) error {
			return di.Shutdown()
		},
	}

	registerFlags(&command.Flags)

	return command
}

func describeQuit(err error) error {
	if err == nil {
		log.Info("stopping application")
	} else {
		log.Errorf("terminating application due to error: %+v\n", err)
	}
	return err
}

// serviceCommand represent entrypoint for service command with top level components
type serviceCommand struct {
	identityHandler identity_selector.Handler
	tequilapi       *client.Client
	errorChannel    chan error
	ap              client.AccessPoliciesRequest
}

// Run runs a command
func (sc *serviceCommand) Run(ctx *cli.Context) (err error) {
	arg := ctx.Args().Get(0)
	if arg != "" {
		serviceTypes = strings.Split(arg, ",")
	}

	providerID := sc.unlockIdentity(parseIdentityFlags(ctx))
	log.Infof("unlocked identity: %v", providerID.Address)

	if err := sc.runServices(ctx, providerID.Address, serviceTypes); err != nil {
		return err
	}

	return <-sc.errorChannel
}

func (sc *serviceCommand) unlockIdentity(identityOptions service.OptionsIdentity) *identity.Identity {
	const retryRate = 10 * time.Second
	for {
		id, err := sc.tequilapi.CurrentIdentity(identityOptions.Identity, identityOptions.Passphrase)
		if err == nil {
			return &identity.Identity{Address: id.Address}
		}
		log.Warnf("failed to get current identity: %v", err)
		log.Warnf("retrying in %vs...", retryRate.Seconds())
		time.Sleep(retryRate)
	}
}

func (sc *serviceCommand) runServices(ctx *cli.Context, providerID string, serviceTypes []string) error {
	for _, serviceType := range serviceTypes {
		options, err := parseFlagsByServiceType(ctx, serviceType)
		if err != nil {
			return err
		}
		go sc.runService(providerID, serviceType, options)
	}

	return nil
}

func (sc *serviceCommand) runService(providerID, serviceType string, options service.Options) {
	_, err := sc.tequilapi.ServiceStart(providerID, serviceType, options, sc.ap)
	if err != nil {
		sc.errorChannel <- errors.Wrapf(err, "failed to run service %s", serviceType)
	}
}

// registerFlags function register service flags to flag list
func registerFlags(flags *[]cli.Flag) {
	*flags = append(*flags,
		agreedTermsConditionsFlag,
		identityFlag,
		identityPassphraseFlag,
	)
	shared.RegisterFlags(flags)
	openvpn_service.RegisterFlags(flags)
	wireguard_service.RegisterFlags(flags)
}

// parseIdentityFlags function fills in service command options from CLI context
func parseIdentityFlags(ctx *cli.Context) service.OptionsIdentity {
	return service.OptionsIdentity{
		Identity:   ctx.String(identityFlag.Name),
		Passphrase: ctx.String(identityPassphraseFlag.Name),
	}
}

func parseFlagsByServiceType(ctx *cli.Context, serviceType string) (service.Options, error) {
	if f, ok := serviceTypesFlagsParser[serviceType]; ok {
		return f(ctx), nil
	}
	return service.OptionsIdentity{}, errors.Errorf("unknown service type: %q", serviceType)
}

func printTermWarning(licenseCommandName string) {
	fmt.Println(metadata.VersionAsSummary(metadata.LicenseCopyright(
		"run program with 'myst "+licenseCommandName+" --"+license.LicenseWarrantyFlag.Name+"' option",
		"run program with 'myst "+licenseCommandName+" --"+license.LicenseConditionsFlag.Name+"' option",
	)))
	fmt.Println()

	fmt.Println("If you agree with these Terms & Conditions, run program again with '--agreed-terms-and-conditions' flag")
}
