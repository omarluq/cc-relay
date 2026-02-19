package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/omarluq/cc-relay/internal/config"
)

// closeConn closes a network connection, logging any error.
// Close errors are often not actionable in read-only contexts.
func closeConn(c net.Conn) {
	if err := c.Close(); err != nil {
		// Log but ignore - connection cleanup is best-effort
		fmt.Fprintf(os.Stderr, "warning: close error: %v\n", err)
	}
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check if cc-relay server is running",
	Long: `Check the health status of a running cc-relay server by querying
its /health endpoint.`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, _ []string) error {
	// Load config to get server listen address
	configPath := cfgFile
	if configPath == "" {
		configPath = findConfigFileForStatus()
	}

	return checkStatusWithConfig(cmd, configPath)
}

// checkStatusWithConfig checks server health using the config at the given path.
func checkStatusWithConfig(cmd *cobra.Command, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	err = checkHealth(cfg.Server.Listen)
	if err != nil {
		cmd.Printf("✗ cc-relay is not running (%s)\n", cfg.Server.Listen)
		return err
	}

	cmd.Printf("✓ cc-relay is running (%s)\n", cfg.Server.Listen)
	return nil
}

// findConfigFileForStatus is a copy of findConfigFile from serve.go.
// Duplicated to avoid shared state between subcommands.

func findConfigFileForStatus() string {
	// Check current directory
	if _, err := os.Stat(defaultConfigFile); err == nil {
		return defaultConfigFile
	}
	// Check ~/.config/cc-relay/
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		p := filepath.Join(home, ".config", "cc-relay", defaultConfigFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return defaultConfigFile
}


// checkHealth performs an HTTP health check against the server's listen address.
// Sends a raw HTTP GET request to /health endpoint without using http.Client.
func checkHealth(listenAddr string) error {
	if listenAddr == "" {
		return fmt.Errorf("server listen address is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("server not reachable: %w", err)
	}
	defer closeConn(conn)

	// Send HTTP GET request directly
	_, err = fmt.Fprintf(conn, "GET /health HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Read response status line
	resp := bufio.NewReader(conn)
	line, err := resp.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Parse status: "HTTP/1.1 200 OK"
	if len(line) >= 12 && line[9:12] == "200" {
		return nil
	}
	return fmt.Errorf("health check failed: %s", line)
}
