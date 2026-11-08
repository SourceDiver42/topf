// Copyright 2026 PostFinance AG
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/postfinance/topf/internal/cmd/omni"
	"github.com/urfave/cli/v3"
)

func newOmniCmd() *cli.Command {
	return &cli.Command{
		Name:  "omni",
		Usage: "integrate with Sidero Omni",
		Commands: []*cli.Command{
			{
				Name:  "template",
				Usage: "render patches into an Omni cluster template",
				Description: `Renders all patches (including templates) for every node and writes an Omni cluster template,
to be applied with 'omnictl cluster template sync'. Each node needs data.omniMachineID set to its Omni machine UUID.
Patches that render identically for all nodes they apply to are placed on the cluster or machine set, others on the individual machine.`,
				Before: noPositionalArgs,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    outputFlag,
						Aliases: []string{"o"},
						Usage:   "file to write the template to (unredacted), instead of stdout",
					},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					t := MustGetRuntime(ctx)

					output := c.String(outputFlag)
					if output == "" {
						return omni.Template(t, t.Writer())
					}

					//nolint:gosec // writing to a user-chosen path is by design
					f, err := os.OpenFile(output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
					if err != nil {
						return fmt.Errorf("failed to create output file: %w", err)
					}

					if err := omni.Template(t, f); err != nil {
						f.Close()

						return err
					}

					return f.Close()
				},
			},
		},
	}
}
