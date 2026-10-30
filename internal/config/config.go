// Package config loads ~/.config/lazypass/config.yml. The layout mirrors the
// subset of lazygit's config that makes sense here (gui.theme, gui.border).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Theme colors are lists like lazygit's: one color plus optional modifiers,
// e.g. [green, bold] or ['#ff00ff', underline]. Empty means "use the default".
type Theme struct {
	ActiveBorderColor               []string `yaml:"activeBorderColor"`
	InactiveBorderColor             []string `yaml:"inactiveBorderColor"`
	OptionsTextColor                []string `yaml:"optionsTextColor"`
	SelectedLineBgColor             []string `yaml:"selectedLineBgColor"`
	InactiveViewSelectedLineBgColor []string `yaml:"inactiveViewSelectedLineBgColor"`
	DefaultFgColor                  []string `yaml:"defaultFgColor"`
}

type Gui struct {
	// Border is one of rounded (default), single, double, bold, hidden.
	Border string `yaml:"border"`
	Theme  Theme  `yaml:"theme"`
}

type Config struct {
	Gui Gui `yaml:"gui"`
}

// DefaultPath is $XDG_CONFIG_HOME/lazypass/config.yml.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "config.yml"
	}
	return filepath.Join(dir, "lazypass", "config.yml")
}

// Load reads path; a missing file yields the zero Config (all defaults).
func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}
