package hh

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestIsAnonPlayer(t *testing.T) {
	for name, want := range map[string]bool{
		"AnonPlayer00": true, "AnonPlayer7": true, "AnonPlayer123": true,
		"AnonPlayer": false, "AnonPlayerX": false, "anonplayer01": false, "Alice": false,
	} {
		if got := IsAnonPlayer(name); got != want {
			t.Errorf("IsAnonPlayer(%q) = %v", name, got)
		}
	}
}

func TestReadPackIDs(t *testing.T) {
	dir := t.TempDir()
	infos := []PackInfo{{Player: "Alice", ID: 1}, {Player: "Bob K", ID: 2}, {Player: "AnonPlayer00", ID: 3, Anon: true}, {Player: "Loner"}}
	for _, info := range infos {
		if err := WritePackFile(filepath.Join(dir, info.Player+".hhpk"), info, slices.Values([]PlayerHand(nil)), CodecNone); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a pack"), 0o644)
	ids, err := ReadPackIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[1] != (PackSummary{infos[0], PackStats{}}) || ids[2] != (PackSummary{infos[1], PackStats{}}) || ids[3] != (PackSummary{infos[2], PackStats{}}) {
		t.Errorf("ids %v", ids)
	}
	if err := WritePackFile(filepath.Join(dir, "Twin.hhpk"), PackInfo{Player: "Twin", ID: 2}, slices.Values([]PlayerHand(nil)), CodecNone); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPackIDs(dir); err == nil {
		t.Error("two packs with ID 2 accepted")
	}
}
