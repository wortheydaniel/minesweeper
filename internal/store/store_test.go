package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestFirstRunDefaults(t *testing.T) {
	s := Open(t.TempDir())
	if s.Notice != "" || !s.Data.Settings.ClickNumberChords || s.Data.Settings.Theme != "classic" || s.Data.LastBoard.W != 9 {
		t.Fatalf("bad defaults: %+v %q", s.Data, s.Notice)
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	s.Data.Settings.Theme = "dark"
	s.Data.Settings.Scale = 3
	s.Data.LastBoard = Board{16, 16, 40}
	rank := s.AddRecord("expert", 61234, now)
	s.Rename("expert", rank, "Ada")
	s2 := Open(dir)
	if s2.Data.Settings.Theme != "dark" || s2.Data.Settings.Scale != 3 || s2.Data.LastBoard.W != 16 {
		t.Fatalf("settings lost: %+v", s2.Data)
	}
	r := s2.Data.BestTimes["expert"]
	if len(r) != 1 || r[0].Name != "Ada" || r[0].MS != 61234 || s2.Data.LastName != "Ada" {
		t.Fatalf("records lost: %+v", r)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestRankingAndTieBreak(t *testing.T) {
	s := Open(t.TempDir())
	for _, ms := range []int64{50, 30, 40, 20, 10} {
		if s.AddRecord("beginner", ms, now) == 0 {
			t.Fatalf("%d should make the list", ms)
		}
	}
	if got := s.AddRecord("beginner", 60, now); got != 0 {
		t.Fatalf("60 should not qualify, got rank %d", got)
	}
	// ties rank behind the earlier equal time
	if got := s.AddRecord("beginner", 30, now); got != 4 {
		t.Fatalf("tie rank %d, want 4 (behind existing 30)", got)
	}
	rs := s.Data.BestTimes["beginner"]
	want := []int64{10, 20, 30, 30, 40}
	if len(rs) != MaxRecord {
		t.Fatalf("len %d", len(rs))
	}
	for i, w := range want {
		if rs[i].MS != w {
			t.Fatalf("order %v", rs)
		}
	}
}

func TestCorruptFileIsBackedUp(t *testing.T) {
	for name, content := range map[string]string{
		"garbage":       "{not json",
		"wrong version": `{"version":99}`,
		"too big":       strings.Repeat("x", maxFile+10),
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "state.json"), []byte(content), 0o644)
		s := Open(dir)
		if s.Notice != "SAVE RESET" || s.Data.LastBoard.W != 9 {
			t.Fatalf("%s: notice %q", name, s.Notice)
		}
		if bad, _ := filepath.Glob(filepath.Join(dir, "state.json.bad-*")); len(bad) != 1 {
			t.Fatalf("%s: backup not created", name)
		}
	}
}

func TestOutOfRangeValuesAreClamped(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"version":1,
	 "settings":{"theme":"neon","scale":99},
	 "lastBoard":{"w":1000,"h":2,"mines":0},
	 "lastName":"  éé Bob\u0007  ",
	 "bestTimes":{"beginner":[{"name":"a","ms":9},{"name":"b","ms":1},{"name":"c","ms":5},{"name":"d","ms":6},{"name":"e","ms":7},{"name":"f","ms":8}]}}`), 0o644)
	s := Open(dir)
	d := s.Data
	if d.Settings.Theme != "classic" || d.Settings.Scale != 0 || d.LastBoard.W != 9 || d.LastName != "Bob" {
		t.Fatalf("not cleaned: %+v", d)
	}
	r := d.BestTimes["beginner"]
	if len(r) != 5 || r[0].MS != 1 || r[4].MS != 8 {
		t.Fatalf("records not sorted/trimmed: %+v", r)
	}
}

func TestUnwritableDirStillWorks(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	os.WriteFile(f, nil, 0o644)
	s := Open(filepath.Join(f, "sub")) // parent is a file: cannot create
	if s.Notice != "CANT SAVE" {
		t.Fatalf("notice %q", s.Notice)
	}
	if s.AddRecord("beginner", 10, now) != 1 || s.Save() != nil {
		t.Fatal("must keep working in memory")
	}
}

func TestNameSanitising(t *testing.T) {
	s := Open(t.TempDir())
	rank := s.AddRecord("beginner", 1, now)
	s.Rename("beginner", rank, "   ")
	if s.Data.BestTimes["beginner"][0].Name != "Anonymous" {
		t.Fatal("blank name should become Anonymous")
	}
	s.Rename("beginner", rank, strings.Repeat("z", 50))
	if n := s.Data.BestTimes["beginner"][0].Name; len(n) != MaxName {
		t.Fatalf("name length %d", len(n))
	}
}

func TestResetTimes(t *testing.T) {
	s := Open(t.TempDir())
	s.AddRecord("beginner", 1, now)
	s.ResetTimes()
	if len(s.Data.BestTimes["beginner"]) != 0 {
		t.Fatal("not reset")
	}
}
