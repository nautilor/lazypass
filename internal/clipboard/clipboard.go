package clipboard

import (
	"errors"
	"os/exec"
	"strings"
)

var clipCmds = [][]string{
	{"wl-copy"},
	{"xclip", "-selection", "clipboard"},
	{"xsel", "--clipboard", "--input"},
	{"pbcopy"},
}

func Copy(s string) error {
	for _, c := range clipCmds {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Stdin = strings.NewReader(s)
		return cmd.Run()
	}
	return errors.New("no clipboard tool found (wl-copy, xclip, xsel, pbcopy)")
}
