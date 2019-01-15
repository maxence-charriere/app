package core

import (
	"github.com/murlokswarm/app"
)

// Menu is a modular implementation of the app.Menu interface that can be
// configured address the different drivers needs.
type Menu struct {
	Elem

	kind string
}

// Kind satisfies the app.Menu interface.
func (m *Menu) Kind() string {
	return m.kind
}

// Create creates and display the menu.
func (m *Menu) Create(c app.MenuConfig) {

}

// WhenMenu satisfies the app.Menu interface.
func (m *Menu) WhenMenu(f func(app.Menu)) {
	f(m)
}

// Load satisfies the app.Menu interface.
func (m *Menu) Load(url string, v ...interface{}) {
	m.SetErr(app.ErrNotSupported)
}

// Compo satisfies the app.Menu interface.
func (m *Menu) Compo() app.Compo {
	return nil
}

// Contains satisfies the app.Menu interface.
func (m *Menu) Contains(c app.Compo) bool {
	return false
}

// Render satisfies the app.Menu interface.
func (m *Menu) Render(c app.Compo) {
	m.SetErr(app.ErrNotSupported)
}

// Reload satisfies the app.Menu interface.
func (m *Menu) Reload() {
	m.SetErr(app.ErrNotSupported)
}

// CanPrevious satisfies the app.Menu interface.
func (m *Menu) CanPrevious() bool {
	return false
}

// Previous satisfies the app.Menu interface.
func (m *Menu) Previous() {
	m.SetErr(app.ErrNotSupported)
}

// CanNext satisfies the app.Menu interface.
func (m *Menu) CanNext() bool {
	return false
}

// Next satisfies the app.Menu interface.
func (m *Menu) Next() {
	m.SetErr(app.ErrNotSupported)
}

// Type satisfies the app.Menu interface.
func (m *Menu) Type() string {
	return "menu"
}

// StatusMenu is a base struct to embed in app.StatusMenu implementations.
type StatusMenu struct {
	Menu
}

// WhenStatusMenu satisfies the app.StatusMenu interface.
func (s *StatusMenu) WhenStatusMenu(f func(app.StatusMenu)) {
	f(s)
}

// Type satisfies the app.Menu interface.
func (s *StatusMenu) Type() string {
	return "status menu"
}

// SetIcon satisfies the app.StatusMenu interface.
func (s *StatusMenu) SetIcon(path string) {
	s.SetErr(app.ErrNotSupported)
}

// SetText satisfies the app.StatusMenu interface.
func (s *StatusMenu) SetText(text string) {
	s.SetErr(app.ErrNotSupported)
}

// Close satisfies the app.StatusMenu interface.
func (s *StatusMenu) Close() {
	s.SetErr(app.ErrNotSupported)
}

// DockTile is a base struct to embed in app.DockTile implementations.
type DockTile struct {
	Menu
}

// WhenDockTile satisfies the app.DockTile interface.
func (d *DockTile) WhenDockTile(f func(app.DockTile)) {
	f(d)
}

// Type satisfies the app.DockTile interface.
func (d *DockTile) Type() string {
	return "dock tile"
}

// SetIcon satisfies the app.DockTile interface.
func (d *DockTile) SetIcon(path string) {
	d.SetErr(app.ErrNotSupported)
}

// SetBadge satisfies the app.DockTile interface.
func (d *DockTile) SetBadge(v interface{}) {
	d.SetErr(app.ErrNotSupported)
}
