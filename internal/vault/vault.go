package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Entry is a single credential.
type Entry struct {
	Name     string    `json:"name"`
	Username string    `json:"username"`
	Password string    `json:"password"`
	URL      string    `json:"url"`
	Notes    string    `json:"notes"`
	Updated  time.Time `json:"updated"`
}

// Workspace is a named, independently encrypted set of entries.
type Workspace struct {
	Name    string
	Entries []Entry

	dataKey []byte
}

type fileFormat struct {
	Version    int    `json:"version"`
	WrappedKey []byte `json:"wrapped_key"` // RSA-OAEP(dataKey)
	Data       []byte `json:"data"`        // nonce || AES-256-GCM(entries)
}

// Store manages workspaces on disk.
type Store struct {
	Dir  string
	keys *Keys
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]{0,63}$`)

func NewStore(dir string, keys *Keys) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{Dir: dir, keys: keys}, nil
}

func (s *Store) path(name string) string { return filepath.Join(s.Dir, name+".ws") }

// List returns workspace names, sorted.
func (s *Store) List() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(s.Dir, "*.ws"))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, strings.TrimSuffix(filepath.Base(f), ".ws"))
	}
	sort.Strings(names)
	return names, nil
}

// Create makes a new empty workspace with a fresh data key.
func (s *Store) Create(name string) (*Workspace, error) {
	if !nameRe.MatchString(name) {
		return nil, errors.New("invalid workspace name")
	}
	if _, err := os.Stat(s.path(name)); err == nil {
		return nil, errors.New("workspace already exists")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	ws := &Workspace{Name: name, dataKey: key}
	return ws, s.Save(ws)
}

// Open decrypts a workspace using the RSA private key.
func (s *Store) Open(name string) (*Workspace, error) {
	b, err := os.ReadFile(s.path(name))
	if err != nil {
		return nil, err
	}
	var ff fileFormat
	if err := json.Unmarshal(b, &ff); err != nil {
		return nil, err
	}
	key, err := rsa.DecryptOAEP(sha256.New(), nil, s.keys.Priv, ff.WrappedKey, []byte(name))
	if err != nil {
		return nil, fmt.Errorf("cannot unwrap key (wrong ssh key?): %w", err)
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(ff.Data) < gcm.NonceSize() {
		return nil, errors.New("corrupt workspace file")
	}
	plain, err := gcm.Open(nil, ff.Data[:gcm.NonceSize()], ff.Data[gcm.NonceSize():], []byte(name))
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}
	ws := &Workspace{Name: name, dataKey: key}
	if err := json.Unmarshal(plain, &ws.Entries); err != nil {
		return nil, err
	}
	return ws, nil
}

// Save encrypts and atomically writes the workspace.
func (s *Store) Save(ws *Workspace) error {
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &s.keys.Priv.PublicKey, ws.dataKey, []byte(ws.Name))
	if err != nil {
		return err
	}
	gcm, err := newGCM(ws.dataKey)
	if err != nil {
		return err
	}
	plain, err := json.Marshal(ws.Entries)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	data := gcm.Seal(nonce, nonce, plain, []byte(ws.Name))
	out, err := json.Marshal(fileFormat{Version: 1, WrappedKey: wrapped, Data: data})
	if err != nil {
		return err
	}
	tmp := s.path(ws.Name) + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(ws.Name))
}

// Delete removes a workspace file.
func (s *Store) Delete(name string) error { return os.Remove(s.path(name)) }

// Rename moves a workspace; re-encrypts since name is bound as AAD.
func (s *Store) Rename(ws *Workspace, newName string) error {
	if !nameRe.MatchString(newName) {
		return errors.New("invalid workspace name")
	}
	if _, err := os.Stat(s.path(newName)); err == nil {
		return errors.New("workspace already exists")
	}
	old := ws.Name
	ws.Name = newName
	if err := s.Save(ws); err != nil {
		ws.Name = old
		return err
	}
	return os.Remove(s.path(old))
}

func newGCM(key []byte) (cipher.AEAD, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(blk)
}

// Generate returns a random password of length n.
func Generate(n int) (string, error) {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%^&*-_=+"
	out := make([]byte, n)
	max := byte(256 - 256%len(alphabet))
	buf := make([]byte, 1)
	for i := 0; i < n; {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		if buf[0] >= max {
			continue
		}
		out[i] = alphabet[int(buf[0])%len(alphabet)]
		i++
	}
	return string(out), nil
}
