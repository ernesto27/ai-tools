package main

import (
	"github.com/spf13/cobra"

	"soft-factory/internal/config"
	"soft-factory/internal/sandbox"
)

func newAuditCmd() *cobra.Command {
	var logPath string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Create audit html for log file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			configPath, err := configPathFrom(cmd)
			if err != nil {
				return err
			}
			if configPath == "" {
				configPath = factoryConfigFile
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			return sandbox.Audit(logPath, cfg)
		},
	}

	cmd.Flags().StringVar(
		&logPath,
		"log",
		"",
		"Absolute path to the execution log",
	)

	addConfigFlag(cmd)

	if err := cmd.MarkFlagRequired("log"); err != nil {
		panic(err)
	}

	return cmd
}
