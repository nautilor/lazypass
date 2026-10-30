package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jesseduffield/gocui"

	"lazypass/internal/config"
)

type theme struct {
	active, inactive, options   gocui.Attribute
	selBg, inactiveSelBg, defFg gocui.Attribute
	frame                       []rune
}

var colorNames = map[string]gocui.Attribute{
	"default": gocui.ColorDefault,
	"black":   gocui.ColorBlack,
	"red":     gocui.ColorRed,
	"green":   gocui.ColorGreen,
	"yellow":  gocui.ColorYellow,
	"blue":    gocui.ColorBlue,
	"magenta": gocui.ColorMagenta,
	"cyan":    gocui.ColorCyan,
	"white":   gocui.ColorWhite,
}

var modifiers = map[string]gocui.Attribute{
	"bold":          gocui.AttrBold,
	"dim":           gocui.AttrDim,
	"reverse":       gocui.AttrReverse,
	"underline":     gocui.AttrUnderline,
	"strikethrough": gocui.AttrStrikeThrough,
}

var borders = map[string][]rune{
	"rounded": {'─', '│', '╭', '╮', '╰', '╯'},
	"single":  {'─', '│', '┌', '┐', '└', '┘'},
	"double":  {'═', '║', '╔', '╗', '╚', '╝'},
	"bold":    {'━', '┃', '┏', '┓', '┗', '┛'},
	"hidden":  {' ', ' ', ' ', ' ', ' ', ' '},
}

var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// parseColor turns ["green","bold"] into one Attribute; empty falls back to def.
func parseColor(key string, list, def []string) (gocui.Attribute, error) {
	if len(list) == 0 {
		list = def
	}
	attr := gocui.ColorDefault
	var mods gocui.Attribute
	for _, item := range list {
		name := strings.ToLower(strings.TrimSpace(item))
		switch {
		case modifiers[name] != 0:
			mods |= modifiers[name]
		case hexRe.MatchString(name):
			attr = gocui.GetColor(name)
		default:
			c, ok := colorNames[name]
			if !ok {
				return 0, fmt.Errorf("gui.theme.%s: unknown color %q", key, item)
			}
			attr = c
		}
	}
	return attr | mods, nil
}

func newTheme(c config.Config) (*theme, error) {
	t := c.Gui.Theme
	th := &theme{}
	for _, f := range []struct {
		dst  *gocui.Attribute
		key  string
		val  []string
		defv []string
	}{
		{&th.active, "activeBorderColor", t.ActiveBorderColor, []string{"green", "bold"}},
		{&th.inactive, "inactiveBorderColor", t.InactiveBorderColor, []string{"default"}},
		{&th.options, "optionsTextColor", t.OptionsTextColor, []string{"blue"}},
		{&th.selBg, "selectedLineBgColor", t.SelectedLineBgColor, []string{"blue"}},
		{&th.inactiveSelBg, "inactiveViewSelectedLineBgColor", t.InactiveViewSelectedLineBgColor, []string{"#444444"}},
		{&th.defFg, "defaultFgColor", t.DefaultFgColor, []string{"default"}},
	} {
		v, err := parseColor(f.key, f.val, f.defv)
		if err != nil {
			return nil, err
		}
		*f.dst = v
	}
	border := strings.ToLower(c.Gui.Border)
	if border == "" {
		border = "rounded"
	}
	runes, ok := borders[border]
	if !ok {
		return nil, fmt.Errorf("gui.border: unknown style %q (rounded|single|double|bold|hidden)", c.Gui.Border)
	}
	th.frame = runes
	return th, nil
}
