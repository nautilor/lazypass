package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jesseduffield/gocui"

	"lazypass/internal/clipboard"
	"lazypass/internal/vault"
)

const (
	vWorkspaces = "workspaces"
	vEntries    = "entries"
	vDetail     = "detail"
	vStatus     = "status"
	vPrompt     = "prompt"
	vConfirm    = "confirm"
	vHelp       = "help"
)

var roundedFrame = []rune{'─', '│', '╭', '╮', '╰', '╯'}

const clipboardTTL = 20 * time.Second

type app struct {
	g     *gocui.Gui
	store *vault.Store

	names  []string // workspace names
	wsSel  int
	ws     *vault.Workspace // opened workspace
	view   []int            // indexes into ws.Entries after filtering
	enSel  int
	filter string
	reveal bool
	msg    string

	focus string
	modal string // name of the active popup, "" if none

	clipTimer *time.Timer
}

// Run starts the TUI.
func Run(store *vault.Store) error {
	g, err := gocui.NewGui(gocui.NewGuiOpts{OutputMode: gocui.OutputTrue, SupportOverlaps: true})
	if err != nil {
		return err
	}
	defer g.Close()

	a := &app{g: g, store: store, focus: vWorkspaces}
	g.Highlight = true
	g.Cursor = false
	g.SelFgColor = gocui.ColorGreen
	g.SelFrameColor = gocui.ColorGreen
	g.SetManagerFunc(a.layout)

	if err := a.bind(); err != nil {
		return err
	}
	if err := a.reloadWorkspaces(""); err != nil {
		return err
	}
	if err := g.MainLoop(); err != nil && !errors.Is(err, gocui.ErrQuit) {
		return err
	}
	return nil
}

// ---------- layout ----------

func (a *app) layout(g *gocui.Gui) error {
	w, h := g.Size()
	split := w / 3
	if split < 24 {
		split = min(24, w/2)
	}
	wsH := min(len(a.names)+2, max(h/4, 5))
	if wsH < 4 {
		wsH = 4
	}

	setup := func(name string, x0, y0, x1, y1 int, title string) (*gocui.View, error) {
		v, err := g.SetView(name, x0, y0, x1, y1, 0)
		if err != nil && !isUnknownView(err) {
			return nil, err
		}
		v.Title = title
		v.FrameRunes = roundedFrame
		return v, nil
	}

	v, err := setup(vWorkspaces, 0, 0, split-1, wsH-1, " [1] Workspaces ")
	if err != nil {
		return err
	}
	v.Highlight = true
	v.SelBgColor = gocui.ColorBlue
	a.renderWorkspaces(v)

	title := " [2] Entries "
	if a.ws != nil {
		title = fmt.Sprintf(" [2] Entries — %s ", a.ws.Name)
	}
	if a.filter != "" {
		title += fmt.Sprintf("(filter: %s) ", a.filter)
	}
	v, err = setup(vEntries, 0, wsH, split-1, h-3, title)
	if err != nil {
		return err
	}
	v.Highlight = true
	v.SelBgColor = gocui.ColorBlue
	a.renderEntries(v)

	v, err = setup(vDetail, split, 0, w-1, h-3, " Details ")
	if err != nil {
		return err
	}
	v.Wrap = true
	a.renderDetail(v)

	v, err = g.SetView(vStatus, -1, h-3, w, h-1, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	v.Frame = false
	a.renderStatus(v)

	if a.modal == "" {
		if _, err := g.SetCurrentView(a.focus); err != nil {
			return err
		}
	}
	return nil
}

// ---------- rendering ----------

const (
	cGreen = "\x1b[32m"
	cCyan  = "\x1b[36m"
	cDim   = "\x1b[2m"
	cBold  = "\x1b[1m"
	cYel   = "\x1b[33m"
	cRed   = "\x1b[31m"
	cReset = "\x1b[0m"
)

func (a *app) renderWorkspaces(v *gocui.View) {
	v.Clear()
	for _, n := range a.names {
		fmt.Fprintln(v, n)
	}
	v.SetCursor(0, 0)
	v.SetOrigin(0, 0)
	a.place(v, a.wsSel)
}

func (a *app) renderEntries(v *gocui.View) {
	v.Clear()
	if a.ws == nil {
		fmt.Fprintln(v, cDim+"no workspace — press n"+cReset)
		return
	}
	for _, i := range a.view {
		fmt.Fprintln(v, a.ws.Entries[i].Name)
	}
	if len(a.view) == 0 {
		fmt.Fprintln(v, cDim+"empty — press a to add"+cReset)
	}
	a.place(v, a.enSel)
}

// place puts the highlighted line at index sel, scrolling if needed.
func (a *app) place(v *gocui.View, sel int) {
	_, h := v.Size()
	if h < 1 {
		h = 1
	}
	oy := 0
	if sel >= h {
		oy = sel - h + 1
	}
	v.SetOrigin(0, oy)
	v.SetCursor(0, sel-oy)
}

func (a *app) renderDetail(v *gocui.View) {
	v.Clear()
	e := a.current()
	if e == nil {
		fmt.Fprintln(v, cDim+"nothing selected"+cReset)
		return
	}
	pw := strings.Repeat("•", min(len([]rune(e.Password)), 16))
	if a.reveal {
		pw = e.Password
	}
	row := func(k, val string) { fmt.Fprintf(v, "%s%-10s%s %s\n", cCyan, k, cReset, val) }
	fmt.Fprintf(v, "%s%s%s\n\n", cBold+cGreen, e.Name, cReset)
	row("Username", e.Username)
	row("Password", pw)
	row("URL", e.URL)
	row("Updated", e.Updated.Local().Format("2006-01-02 15:04"))
	if e.Notes != "" {
		fmt.Fprintf(v, "\n%sNotes%s\n%s\n", cCyan, cReset, e.Notes)
	}
}

func (a *app) renderStatus(v *gocui.View) {
	v.Clear()
	var hints string
	switch a.focus {
	case vWorkspaces:
		hints = "n: new  r: rename  d: delete  enter: open  tab: switch  ?: help  q: quit"
	default:
		hints = "a: add  e: edit  d: delete  c: copy pw  u: copy user  space: reveal  /: filter  g: generate  ?: help  q: quit"
	}
	fmt.Fprintf(v, " %s%s%s", cCyan, hints, cReset)
	if a.msg != "" {
		fmt.Fprintf(v, "\n %s%s%s", cYel, a.msg, cReset)
	}
}

func (a *app) setMsg(s string) {
	a.msg = s
	if a.g != nil {
		time.AfterFunc(4*time.Second, func() {
			a.g.Update(func(*gocui.Gui) error {
				if a.msg == s {
					a.msg = ""
				}
				return nil
			})
		})
	}
}

// ---------- state ----------

func (a *app) current() *vault.Entry {
	if a.ws == nil || a.enSel < 0 || a.enSel >= len(a.view) {
		return nil
	}
	return &a.ws.Entries[a.view[a.enSel]]
}

func (a *app) reloadWorkspaces(want string) error {
	names, err := a.store.List()
	if err != nil {
		return err
	}
	a.names = names
	a.wsSel = 0
	for i, n := range names {
		if n == want {
			a.wsSel = i
		}
	}
	if len(names) == 0 {
		a.ws = nil
		a.view = nil
		return nil
	}
	return a.openSelected()
}

func (a *app) openSelected() error {
	if a.wsSel >= len(a.names) {
		return nil
	}
	ws, err := a.store.Open(a.names[a.wsSel])
	if err != nil {
		a.ws = nil
		a.view = nil
		a.setMsg("error: " + err.Error())
		return nil
	}
	a.ws = ws
	a.filter = ""
	a.enSel = 0
	a.reveal = false
	a.refilter()
	return nil
}

func (a *app) refilter() {
	a.view = a.view[:0]
	if a.ws == nil {
		return
	}
	q := strings.ToLower(a.filter)
	for i, e := range a.ws.Entries {
		if q == "" || strings.Contains(strings.ToLower(e.Name+" "+e.Username+" "+e.URL), q) {
			a.view = append(a.view, i)
		}
	}
	if a.enSel >= len(a.view) {
		a.enSel = max(len(a.view)-1, 0)
	}
}

func (a *app) save() {
	if err := a.store.Save(a.ws); err != nil {
		a.setMsg("save failed: " + err.Error())
	}
}

// ---------- keybindings ----------

func (a *app) bind() error {
	g := a.g
	type kb struct {
		view string
		key  any
		fn   func(*gocui.Gui, *gocui.View) error
	}
	only := func(fn func() error) func(*gocui.Gui, *gocui.View) error {
		return func(*gocui.Gui, *gocui.View) error {
			if a.modal != "" {
				return nil
			}
			return fn()
		}
	}
	list := []kb{
		{"", gocui.KeyCtrlC, func(*gocui.Gui, *gocui.View) error { return gocui.ErrQuit }},
		{"", 'q', only(func() error { return gocui.ErrQuit })},
		{"", '?', only(a.showHelp)},
		{"", gocui.KeyTab, only(a.toggleFocus)},
		{"", '1', only(func() error { a.focus = vWorkspaces; return nil })},
		{"", '2', only(func() error { a.focus = vEntries; return nil })},
		{"", 'h', only(func() error { a.focus = vWorkspaces; return nil })},
		{"", 'l', only(func() error { a.focus = vEntries; return nil })},
		{vWorkspaces, 'j', only(func() error { return a.moveWS(1) })},
		{vWorkspaces, gocui.KeyArrowDown, only(func() error { return a.moveWS(1) })},
		{vWorkspaces, 'k', only(func() error { return a.moveWS(-1) })},
		{vWorkspaces, gocui.KeyArrowUp, only(func() error { return a.moveWS(-1) })},
		{vWorkspaces, gocui.KeyEnter, only(func() error { a.focus = vEntries; return nil })},
		{vWorkspaces, 'n', only(a.newWorkspace)},
		{vWorkspaces, 'r', only(a.renameWorkspace)},
		{vWorkspaces, 'd', only(a.deleteWorkspace)},
		{vEntries, 'j', only(func() error { a.moveEntry(1); return nil })},
		{vEntries, gocui.KeyArrowDown, only(func() error { a.moveEntry(1); return nil })},
		{vEntries, 'k', only(func() error { a.moveEntry(-1); return nil })},
		{vEntries, gocui.KeyArrowUp, only(func() error { a.moveEntry(-1); return nil })},
		{vEntries, 'a', only(func() error { return a.editEntry(nil) })},
		{vEntries, 'e', only(func() error { return a.editEntry(a.current()) })},
		{vEntries, 'd', only(a.deleteEntry)},
		{vEntries, 'c', only(func() error { return a.copyField("password") })},
		{vEntries, gocui.KeyEnter, only(func() error { return a.copyField("password") })},
		{vEntries, 'u', only(func() error { return a.copyField("username") })},
		{vEntries, gocui.KeySpace, only(func() error { a.reveal = !a.reveal; return nil })},
		{vEntries, '/', only(a.askFilter)},
		{vEntries, gocui.KeyEsc, only(func() error { a.filter = ""; a.refilter(); return nil })},
		{vEntries, 'g', only(a.genPassword)},
	}
	for _, k := range list {
		if err := g.SetKeybinding(k.view, k.key, gocui.ModNone, k.fn); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) toggleFocus() error {
	if a.focus == vWorkspaces {
		a.focus = vEntries
	} else {
		a.focus = vWorkspaces
	}
	return nil
}

func (a *app) moveWS(d int) error {
	n := a.wsSel + d
	if n < 0 || n >= len(a.names) {
		return nil
	}
	a.wsSel = n
	return a.openSelected()
}

func (a *app) moveEntry(d int) {
	n := a.enSel + d
	if n < 0 || n >= len(a.view) {
		return
	}
	a.enSel = n
	a.reveal = false
}

// ---------- actions ----------

func (a *app) newWorkspace() error {
	return a.prompt("New workspace", "", false, nil, func(s string) error {
		if _, err := a.store.Create(s); err != nil {
			a.setMsg("error: " + err.Error())
			return nil
		}
		a.focus = vEntries
		return a.reloadWorkspaces(s)
	})
}

func (a *app) renameWorkspace() error {
	if a.ws == nil {
		return nil
	}
	return a.prompt("Rename workspace", a.ws.Name, false, nil, func(s string) error {
		if err := a.store.Rename(a.ws, s); err != nil {
			a.setMsg("error: " + err.Error())
			return nil
		}
		return a.reloadWorkspaces(s)
	})
}

func (a *app) deleteWorkspace() error {
	if a.ws == nil {
		return nil
	}
	name := a.ws.Name
	return a.confirm(fmt.Sprintf("Delete workspace %q and ALL its passwords? (y/n)", name), func() error {
		if err := a.store.Delete(name); err != nil {
			a.setMsg("error: " + err.Error())
			return nil
		}
		return a.reloadWorkspaces("")
	})
}

func (a *app) deleteEntry() error {
	e := a.current()
	if e == nil {
		return nil
	}
	idx := a.view[a.enSel]
	return a.confirm(fmt.Sprintf("Delete entry %q? (y/n)", e.Name), func() error {
		a.ws.Entries = append(a.ws.Entries[:idx], a.ws.Entries[idx+1:]...)
		a.save()
		a.refilter()
		return nil
	})
}

func (a *app) askFilter() error {
	if a.ws == nil {
		return nil
	}
	return a.prompt("Filter", a.filter, false, nil, func(s string) error {
		a.filter = s
		a.enSel = 0
		a.refilter()
		return nil
	})
}

// editEntry walks through one prompt per field. e == nil adds a new entry.
func (a *app) editEntry(e *vault.Entry) error {
	if a.ws == nil {
		a.setMsg("create a workspace first")
		return nil
	}
	var cur vault.Entry
	if e != nil {
		cur = *e
	}
	fields := []struct {
		title string
		val   *string
		mask  bool
	}{
		{"Name", &cur.Name, false},
		{"Username", &cur.Username, false},
		{"Password (ctrl+g: generate)", &cur.Password, true},
		{"URL", &cur.URL, false},
		{"Notes", &cur.Notes, false},
	}
	idx := -1
	if e != nil {
		idx = a.view[a.enSel]
	}
	var step func(i int) error
	step = func(i int) error {
		if i == len(fields) {
			if strings.TrimSpace(cur.Name) == "" {
				a.setMsg("name required")
				return nil
			}
			cur.Updated = time.Now()
			if idx >= 0 {
				a.ws.Entries[idx] = cur
			} else {
				a.ws.Entries = append(a.ws.Entries, cur)
				a.filter = ""
			}
			a.save()
			a.refilter()
			for vi, ei := range a.view {
				if a.ws.Entries[ei].Name == cur.Name {
					a.enSel = vi
				}
			}
			return nil
		}
		f := fields[i]
		extra := map[gocui.Key]func(*gocui.View){}
		if f.mask {
			extra[gocui.KeyCtrlG] = func(v *gocui.View) {
				if p, err := vault.Generate(24); err == nil {
					v.ClearTextArea()
					v.TextArea.TypeCharacter(p)
					v.RenderTextArea()
				}
			}
		}
		title := fmt.Sprintf("%s (%d/%d)", f.title, i+1, len(fields))
		return a.prompt(title, *f.val, f.mask, extra, func(s string) error {
			*f.val = s
			return step(i + 1)
		})
	}
	return step(0)
}

func (a *app) copyField(which string) error {
	e := a.current()
	if e == nil {
		return nil
	}
	val := e.Password
	if which == "username" {
		val = e.Username
	}
	if err := clipboard.Copy(val); err != nil {
		a.setMsg("error: " + err.Error())
		return nil
	}
	a.setMsg(fmt.Sprintf("copied %s (clears in %ds)", which, int(clipboardTTL.Seconds())))
	if a.clipTimer != nil {
		a.clipTimer.Stop()
	}
	a.clipTimer = time.AfterFunc(clipboardTTL, func() { _ = clipboard.Copy("") })
	return nil
}

func (a *app) genPassword() error {
	p, err := vault.Generate(24)
	if err != nil {
		return nil
	}
	if err := clipboard.Copy(p); err != nil {
		a.setMsg("error: " + err.Error())
		return nil
	}
	a.setMsg("generated password copied to clipboard")
	return nil
}

// ---------- popups ----------

func (a *app) popupRect(h int) (x0, y0, x1, y1 int) {
	w, sh := a.g.Size()
	pw := min(w-4, 60)
	x0 = (w - pw) / 2
	y0 = (sh - h) / 2
	return x0, y0, x0 + pw, y0 + h - 1
}

func (a *app) closePopup(name string) {
	a.g.DeleteViewKeybindings(name)
	_ = a.g.DeleteView(name)
	a.modal = ""
}

func (a *app) prompt(title, initial string, mask bool, extra map[gocui.Key]func(*gocui.View), done func(string) error) error {
	g := a.g
	if a.modal != "" {
		return nil
	}
	x0, y0, x1, y1 := a.popupRect(3)
	v, err := g.SetView(vPrompt, x0, y0, x1, y1, 1)
	if err != nil && !isUnknownView(err) {
		return err
	}
	a.modal = vPrompt
	v.Title = " " + title + " "
	v.FrameRunes = roundedFrame
	v.Editable = true
	v.Editor = gocui.DefaultEditor
	v.Wrap = false
	if mask {
		v.Mask = "*"
	}
	v.TextArea.TypeCharacter(initial)
	v.RenderTextArea()
	g.Cursor = true
	if _, err := g.SetCurrentView(vPrompt); err != nil {
		return err
	}

	finish := func() {
		g.Cursor = false
		a.closePopup(vPrompt)
	}
	_ = g.SetKeybinding(vPrompt, gocui.KeyEsc, gocui.ModNone, func(*gocui.Gui, *gocui.View) error {
		finish()
		return nil
	})
	_ = g.SetKeybinding(vPrompt, gocui.KeyEnter, gocui.ModNone, func(_ *gocui.Gui, v *gocui.View) error {
		text := strings.TrimRight(v.TextArea.GetContent(), "\n")
		finish()
		return done(text)
	})
	for k, fn := range extra {
		fn := fn
		_ = g.SetKeybinding(vPrompt, k, gocui.ModNone, func(_ *gocui.Gui, v *gocui.View) error {
			fn(v)
			return nil
		})
	}
	return nil
}

func (a *app) confirm(msg string, yes func() error) error {
	g := a.g
	if a.modal != "" {
		return nil
	}
	x0, y0, x1, y1 := a.popupRect(3)
	v, err := g.SetView(vConfirm, x0, y0, x1, y1, 1)
	if err != nil && !isUnknownView(err) {
		return err
	}
	a.modal = vConfirm
	v.Title = " Confirm "
	v.FrameRunes = roundedFrame
	v.Wrap = true
	v.Clear()
	fmt.Fprint(v, cRed+msg+cReset)
	if _, err := g.SetCurrentView(vConfirm); err != nil {
		return err
	}
	no := func(*gocui.Gui, *gocui.View) error {
		a.closePopup(vConfirm)
		return nil
	}
	_ = g.SetKeybinding(vConfirm, 'n', gocui.ModNone, no)
	_ = g.SetKeybinding(vConfirm, gocui.KeyEsc, gocui.ModNone, no)
	_ = g.SetKeybinding(vConfirm, 'y', gocui.ModNone, func(*gocui.Gui, *gocui.View) error {
		a.closePopup(vConfirm)
		return yes()
	})
	return nil
}

func (a *app) showHelp() error {
	g := a.g
	lines := []string{
		"Navigation",
		"  tab / h / l / 1 / 2   switch panel",
		"  j / k, arrows         move",
		"",
		"Workspaces",
		"  n new   r rename   d delete   enter open",
		"",
		"Entries",
		"  a add   e edit   d delete",
		"  c / enter  copy password   u copy username",
		"  space  reveal password",
		"  /  filter   esc clear filter",
		"  g  generate password to clipboard",
		"  (ctrl+g in password field generates inline)",
		"",
		"q quit   ? close help",
	}
	x0, y0, x1, y1 := a.popupRect(len(lines) + 2)
	v, err := g.SetView(vHelp, x0, y0, x1, y1, 1)
	if err != nil && !isUnknownView(err) {
		return err
	}
	a.modal = vHelp
	v.Title = " Help "
	v.FrameRunes = roundedFrame
	v.Clear()
	fmt.Fprint(v, strings.Join(lines, "\n"))
	if _, err := g.SetCurrentView(vHelp); err != nil {
		return err
	}
	closeFn := func(*gocui.Gui, *gocui.View) error {
		a.closePopup(vHelp)
		return nil
	}
	for _, k := range []any{gocui.KeyEsc, '?', 'q', gocui.KeyEnter} {
		_ = g.SetKeybinding(vHelp, k, gocui.ModNone, closeFn)
	}
	return nil
}

// gocui wraps ErrUnknownView in a go-errors value that errors.Is can't see through.
func isUnknownView(err error) bool {
	return err != nil && err.Error() == gocui.ErrUnknownView.Error()
}
