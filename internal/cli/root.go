// Package cli implements the `metadata` command line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/codingcoffee/openmetadata-ingestion-go/internal/config"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/logging"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/sink"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/source"
	"github.com/codingcoffee/openmetadata-ingestion-go/internal/workflow"

	// Register the supported sources via their package init functions.
	_ "github.com/codingcoffee/openmetadata-ingestion-go/internal/source/clickhouse"
	_ "github.com/codingcoffee/openmetadata-ingestion-go/internal/source/mysql"
	_ "github.com/codingcoffee/openmetadata-ingestion-go/internal/source/postgres"
)

// version is set via -ldflags at build time.
var version = "dev"

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "omingest",
		Short:         "Push database metadata to OpenMetadata",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newIngestCmd(), newTestConnectionCmd(), newVersionCmd())
	return root
}

func newIngestCmd() *cobra.Command {
	var configPath string
	var logLevelOverride string

	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Run an ingestion workflow from a YAML config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			wf, err := config.Load(configPath)
			if err != nil {
				return err
			}

			level := wf.WorkflowConfig.LoggerLevel
			if logLevelOverride != "" {
				level = logLevelOverride
			}
			log := logging.Setup(level)

			src, err := source.New(wf.Source)
			if err != nil {
				return err
			}

			snk, err := sink.NewMetadataREST(wf.WorkflowConfig.OpenMetadataServerConfig)
			if err != nil {
				return err
			}

			stats, err := workflow.New(src, snk, log).Run(cmd.Context())
			if err != nil {
				return err
			}
			log.Info("ingestion complete",
				"service", wf.Source.ServiceName,
				"databases", stats.Databases,
				"schemas", stats.Schemas,
				"tables", stats.Tables,
			)
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "path to the workflow YAML config (required)")
	cmd.Flags().StringVar(&logLevelOverride, "log-level", "", "override loggerLevel (DEBUG|INFO|WARN|ERROR)")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func newTestConnectionCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "test-connection",
		Short: "Validate that the source database is reachable",
		RunE: func(cmd *cobra.Command, _ []string) error {
			wf, err := config.Load(configPath)
			if err != nil {
				return err
			}
			src, err := source.New(wf.Source)
			if err != nil {
				return err
			}
			defer src.Close()
			if err := src.Prepare(cmd.Context()); err != nil {
				return fmt.Errorf("connection failed: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "connection to %q OK\n", wf.Source.ServiceName)
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "path to the workflow YAML config (required)")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), version)
			return nil
		},
	}
}
