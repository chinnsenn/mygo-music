package app

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Toggling Cross-Source Lyrics in the Experiments settings saves it, and the
// settings read back from the file say what was chosen.
func TestLyricFallbackSetting(t *testing.T) {
	dir := t.TempDir()
	was, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		t.Fatal(err)
	}
	mygo.App.SetPath(mygo.PathUserData, dir)
	t.Cleanup(func() { mygo.App.SetPath(mygo.PathUserData, was) })

	a := newTestApp()
	path, err := settingsPath()
	if err != nil {
		t.Fatal(err)
	}
	a.settings.path = path
	a.settings.LyricFallback = true
	a.settingsPage = "experiments"
	tt := ui.NewTester(a.settingsView, 780, 1400)
	settled(tt)
	for _, want := range []bool{false, true} {
		n, ok := tt.AccessByRole(ui.RoleSwitch, T("Cross-Source Lyrics"))
		if !ok {
			t.Fatalf("no Cross-Source Lyrics switch: %q", tt.Texts())
		}
		r := n.Bounds
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
		tt.Frame()
		if a.settings.LyricFallback != want {
			t.Fatalf("toggled: LyricFallback %v, want %v", a.settings.LyricFallback, want)
		}
		if got := loadSettings(); got.LyricFallback != want {
			t.Fatalf("read back: LyricFallback %v, want %v", got.LyricFallback, want)
		}
	}
}

// lyricTestApp is an app whose Experiments page writes settings to a file of
// its own, as opening the settings does.
func lyricTestApp(t *testing.T) *app {
	t.Helper()
	dir := t.TempDir()
	was, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		t.Fatal(err)
	}
	mygo.App.SetPath(mygo.PathUserData, dir)
	t.Cleanup(func() { mygo.App.SetPath(mygo.PathUserData, was) })

	a := newTestApp()
	path, err := settingsPath()
	if err != nil {
		t.Fatal(err)
	}
	a.settings.path = path
	a.settings.LyricFallback = true
	a.settingsPage = "experiments"
	return a
}

// clickLyricRole clicks the first control of a role the view labels label.
func clickLyricRole(t *testing.T, tt *ui.Tester, role ui.Role, label string) {
	t.Helper()
	n, ok := tt.AccessByRole(role, label)
	if !ok {
		t.Fatalf("missing %v %q", role, label)
	}
	r := n.Bounds
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
}

// chooseLyricOption picks option in the select labeled label.
func chooseLyricOption(t *testing.T, tt *ui.Tester, label, option string) {
	t.Helper()
	clickLyricRole(t, tt, ui.RolePopUpButton, label)
	clickLyricRole(t, tt, ui.RoleOption, option)
}

// Picking a source another slot holds swaps the two, and None may repeat
// without moving the others; both are saved.
func TestLyricSourceSlotsSwap(t *testing.T) {
	a := lyricTestApp(t)
	a.settings.LyricConfig.Sources = [3]string{"netease", "qq", "kugou"}
	tt := ui.NewTester(a.settingsView, 780, 2000)
	settled(tt)

	chooseLyricOption(t, tt, T("First Lyric Source"), T("QQ Music"))
	if got := a.settings.LyricConfig.Sources; got != [3]string{"qq", "netease", "kugou"} {
		t.Fatalf("swapped: %q", got)
	}
	if got := loadSettings().LyricConfig.Sources; got != [3]string{"qq", "netease", "kugou"} {
		t.Fatalf("swapped back: %q", got)
	}

	chooseLyricOption(t, tt, T("First Lyric Source"), T("None"))
	if got := a.settings.LyricConfig.Sources; got != [3]string{"", "netease", "kugou"} {
		t.Fatalf("None: %q", got)
	}
	if got := loadSettings().LyricConfig.Sources; got != [3]string{"", "netease", "kugou"} {
		t.Fatalf("None back: %q", got)
	}
}

// A changed lookup timeout and a cache turned off are written as chosen, and
// zero stays zero when the settings are read back.
func TestLyricLookupAndCacheSettings(t *testing.T) {
	a := lyricTestApp(t)
	a.settings.LyricConfig.TimeoutSeconds = 12
	a.settings.LyricConfig.CacheEntries = 512
	tt := ui.NewTester(a.settingsView, 780, 2000)
	settled(tt)

	chooseLyricOption(t, tt, T("Lookup Timeout"), Tf("%d seconds", 20))
	chooseLyricOption(t, tt, T("Cache Capacity"), T("Off"))

	got := loadSettings().LyricConfig
	if got.TimeoutSeconds != 20 {
		t.Fatalf("timeout %d s, want 20", got.TimeoutSeconds)
	}
	if got.CacheEntries != 0 {
		t.Fatalf("capacity %d, want off", got.CacheEntries)
	}
}

// Disabling cross-source lyrics hides its controls, including search results;
// enabling it again restores the listener's saved parameters.
func TestLyricSettingsWhileOff(t *testing.T) {
	a := lyricTestApp(t)
	tt := ui.NewTester(a.settingsView, 780, 2000)
	settled(tt)
	chooseLyricOption(t, tt, T("Lookup Timeout"), Tf("%d seconds", 20))
	clickLyricRole(t, tt, ui.RoleSwitch, T("Cross-Source Lyrics"))
	settled(tt)
	if _, ok := tt.AccessByRole(ui.RolePopUpButton, T("Lookup Timeout")); ok {
		t.Fatal("disabled lyric parameters remain interactive")
	}
	a.settingsQuery = T("Lookup Timeout")
	settled(tt)
	if _, ok := tt.AccessByRole(ui.RolePopUpButton, T("Lookup Timeout")); ok {
		t.Fatal("search exposes disabled lyric parameters")
	}
	a.settingsQuery = ""
	settled(tt)
	clickLyricRole(t, tt, ui.RoleSwitch, T("Cross-Source Lyrics"))
	settled(tt)
	chooseLyricOption(t, tt, T("Lookup Timeout"), Tf("%d seconds", 30))
	if got := loadSettings(); !got.LyricFallback || got.LyricConfig.TimeoutSeconds != 30 {
		t.Fatalf("re-enabled parameters were not saved: %+v", got.LyricConfig)
	}
}
