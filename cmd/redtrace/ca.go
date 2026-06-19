package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/utkarshrai2811/redtrace/internal/proxy/cert"
)

var caPrintPEM bool

var caCmd = &cobra.Command{
	Use:   "ca",
	Short: "Generate (if needed) and locate the RedTrace CA certificate",
	Long:  "Ensures the RedTrace CA exists and prints the path to its PEM file,\nwhich you import into your OS or browser trust store to intercept HTTPS.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg := loadConfig()
		authority, err := cert.LoadOrCreateAuthority(cfg.CADir)
		if err != nil {
			return fmt.Errorf("certificate authority: %w", err)
		}
		if caPrintPEM {
			_, err := os.Stdout.Write(authority.CACertPEM())
			return err
		}
		fmt.Println(filepath.Join(cfg.CADir, "redtrace-ca.pem"))
		return nil
	},
}

func init() {
	caCmd.Flags().BoolVar(&caPrintPEM, "pem", false, "print the CA certificate PEM to stdout")
}
