package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/term"

	"lazypass/internal/vault"
)

func main() {
	home, _ := os.UserHomeDir()
	cfg, _ := os.UserConfigDir()
	keyPath := flag.String("key", filepath.Join(home, ".ssh", "id_rsa"), "RSA private key used for encryption")
	dir := flag.String("dir", envOr("LAZYPASS_DIR", filepath.Join(cfg, "lazypass")), "directory holding workspaces")
	flag.Parse()

	keys, err := vault.LoadKey(*keyPath, nil)
	if errors.Is(err, vault.ErrPassphrase) {
		fmt.Fprintf(os.Stderr, "Passphrase for %s: ", *keyPath)
		pw, perr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if perr != nil {
			fatal(perr)
		}
		keys, err = vault.LoadKey(*keyPath, bytes.TrimSpace(pw))
	}
	if err != nil {
		fatal(err)
	}
	store, err := vault.NewStore(*dir, keys)
	if err != nil {
		fatal(err)
	}
	if err := run(store); err != nil {
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
