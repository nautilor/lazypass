package vault

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// Keys holds the RSA keypair used to wrap workspace data keys.
type Keys struct {
	Priv *rsa.PrivateKey
}

// ErrPassphrase is returned when the key is encrypted and no passphrase was given.
var ErrPassphrase = errors.New("ssh key is passphrase protected")

// LoadKey reads an RSA private key (OpenSSH or PEM format).
func LoadKey(path string, passphrase []byte) (*Keys, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw any
	if len(passphrase) > 0 {
		raw, err = ssh.ParseRawPrivateKeyWithPassphrase(pem, passphrase)
	} else {
		raw, err = ssh.ParseRawPrivateKey(pem)
	}
	if err != nil {
		var pm *ssh.PassphraseMissingError
		if errors.As(err, &pm) {
			return nil, ErrPassphrase
		}
		return nil, err
	}
	switch k := raw.(type) {
	case *rsa.PrivateKey:
		return &Keys{Priv: k}, nil
	default:
		return nil, fmt.Errorf("%s is not an RSA key (got %T)", path, raw)
	}
}
