package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"lazypass/internal/ui"
	"lazypass/internal/vault"
)

func main() {
	home, _ := os.UserHomeDir()
	cfg, _ := os.UserConfigDir()
	keyPath := flag.String("key", filepath.Join(home, ".ssh", "id_rsa"), "RSA private key used for encryption")
	dir := flag.String("dir", envOr("LAZYPASS_DIR", filepath.Join(cfg, "lazypass")), "directory holding workspaces")
	flag.Parse()

	// Probe the key: an encrypted one is unlocked from inside the TUI.
	_, err := vault.LoadKey(*keyPath, nil)
	needPass := errors.Is(err, vault.ErrPassphrase)
	if err != nil && !needPass {
		fatal(err)
	}
	unlock := func(pass []byte) (*vault.Store, error) {
		keys, err := vault.LoadKey(*keyPath, pass)
		if err != nil {
			return nil, err
		}
		return vault.NewStore(*dir, keys)
	}
	if err := ui.Run(unlock, needPass); err != nil {
		fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "lazypass:", err)
	os.Exit(1)
}
