// Package store keeps settings and best times in one small JSON file.
// It never stops the game from running: if the file is unusable the game
// simply continues with defaults (and, if needed, without saving).
package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	version   = 1
	maxFile   = 1 << 20 // refuse to parse anything bigger than 1 MiB
	MaxRecord = 5       // best times kept per preset
	MaxName   = 20
)

type Settings struct {
	QuestionMarks     bool   `json:"questionMarks"`
	ClickNumberChords bool   `json:"clickNumberChords"`
	Theme             string `json:"theme"` // "classic" or "dark"
	Scale             int    `json:"scale"` // 0 = auto, otherwise 1-4
}

type Board struct {
	W     int `json:"w"`
	H     int `json:"h"`
	Mines int `json:"mines"`
}

type Record struct {
	Name string    `json:"name"`
	MS   int64     `json:"ms"`
	At   time.Time `json:"at"`
}

type Data struct {
	Version   int                 `json:"version"`
	Settings  Settings            `json:"settings"`
	LastBoard Board               `json:"lastBoard"`
	LastName  string              `json:"lastName"`
	BestTimes map[string][]Record `json:"bestTimes"`
}

// Presets that keep best times.
var Presets = []string{"beginner", "intermediate", "expert"}

func defaults() Data {
	return Data{
		Version:   version,
		Settings:  Settings{ClickNumberChords: true, Theme: "classic"},
		LastBoard: Board{9, 9, 10},
		LastName:  "Anonymous",
		BestTimes: map[string][]Record{},
	}
}

// Store is the loaded state plus where to save it.
type Store struct {
	Data   Data
	Notice string // short message for the UI, empty when all is well
	path   string
}

// DefaultDir is <user config dir>/minesweeper.
func DefaultDir() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "minesweeper"), nil
}

// Open loads dir/state.json (dir "" = DefaultDir). It never fails: problems
// are reported through Notice and the store falls back to defaults.
func Open(dir string) *Store {
	s := &Store{Data: defaults()}
	if dir == "" {
		var err error
		if dir, err = DefaultDir(); err != nil {
			s.Notice = "CANT SAVE"
			return s
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.Notice = "CANT SAVE"
		return s
	}
	s.path = filepath.Join(dir, "state.json")
	f, err := os.Open(s.path)
	if err != nil {
		return s // first run
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	f.Close()
	var d Data
	if err != nil || len(raw) > maxFile || json.Unmarshal(raw, &d) != nil || d.Version != version {
		os.Rename(s.path, fmt.Sprintf("%s.bad-%d", s.path, time.Now().Unix()))
		s.Notice = "SAVE RESET"
		return s
	}
	s.Data = clean(d)
	return s
}

// clean fills gaps and clamps anything out of range, so old or hand-edited
// files can never produce an invalid state.
func clean(d Data) Data {
	def := defaults()
	if d.Settings.Theme != "classic" && d.Settings.Theme != "dark" {
		d.Settings.Theme = def.Settings.Theme
	}
	if d.Settings.Scale < 0 || d.Settings.Scale > 4 {
		d.Settings.Scale = 0
	}
	b := d.LastBoard
	if b.W < 9 || b.W > 50 || b.H < 9 || b.H > 30 || b.Mines < 1 || b.Mines > b.W*b.H-9 {
		d.LastBoard = def.LastBoard
	}
	d.LastName = sanitizeName(d.LastName)
	if d.LastName == "" {
		d.LastName = def.LastName
	}
	best := map[string][]Record{}
	for _, p := range Presets {
		rs := append([]Record(nil), d.BestTimes[p]...)
		sort.SliceStable(rs, func(i, j int) bool { return rs[i].MS < rs[j].MS })
		if len(rs) > MaxRecord {
			rs = rs[:MaxRecord]
		}
		for i := range rs {
			rs[i].Name = sanitizeName(rs[i].Name)
		}
		best[p] = rs
	}
	d.BestTimes = best
	d.Version = version
	return d
}

// SanitizeName trims and keeps printable ASCII, at most MaxName characters.
func sanitizeName(n string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(n) {
		if r >= 0x20 && r < 0x7f && b.Len() < MaxName {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// Dir is the directory holding the state file (empty if saving is off).
func (s *Store) Dir() string {
	if s.path == "" {
		return ""
	}
	return filepath.Dir(s.path)
}

// Save writes the state atomically (temp file, fsync, rename). A failure sets
// Notice but is otherwise harmless.
func (s *Store) Save() error {
	if s.path == "" {
		return nil
	}
	err := s.write()
	if err != nil {
		s.Notice = "CANT SAVE"
	}
	return err
}

func (s *Store) write() error {
	raw, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "state-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, s.path)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}

// AddRecord inserts a win into the preset's top list. It returns the 1-based
// rank, or 0 if the time did not make the top MaxRecord. Equal times rank
// behind earlier ones.
func (s *Store) AddRecord(preset string, ms int64, at time.Time) int {
	rs := s.Data.BestTimes[preset]
	pos := sort.Search(len(rs), func(i int) bool { return rs[i].MS > ms })
	if pos >= MaxRecord {
		return 0
	}
	rs = append(rs, Record{})
	copy(rs[pos+1:], rs[pos:])
	rs[pos] = Record{Name: s.Data.LastName, MS: ms, At: at}
	if len(rs) > MaxRecord {
		rs = rs[:MaxRecord]
	}
	s.Data.BestTimes[preset] = rs
	s.Save()
	return pos + 1
}

// Rename sets the name on a record (rank is 1-based) and remembers it as the
// default for next time.
func (s *Store) Rename(preset string, rank int, name string) {
	name = sanitizeName(name)
	if name == "" {
		name = "Anonymous"
	}
	rs := s.Data.BestTimes[preset]
	if rank >= 1 && rank <= len(rs) {
		rs[rank-1].Name = name
	}
	s.Data.LastName = name
	s.Save()
}

// ResetTimes clears all best times.
func (s *Store) ResetTimes() {
	s.Data.BestTimes = map[string][]Record{}
	s.Save()
}
