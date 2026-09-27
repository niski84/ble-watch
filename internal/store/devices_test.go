package store

import (
	"path/filepath"
	"testing"
)

func TestRecognizedProjectionMatchesDeviceReader(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	const mac = "02:00:00:00:00:01"
	if err := st.UpsertDevice(mac, "Synthetic sensor", "random", "", 1700000000, -65); err != nil {
		t.Fatal(err)
	}
	if err := st.SetGroup(mac, "Demo sensors"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetRecognized(mac, "test", true); err != nil {
		t.Fatal(err)
	}
	all, err := st.ListDevices()
	if err != nil {
		t.Fatal(err)
	}
	recognized, err := st.ListRecognized()
	if err != nil || len(recognized) != 1 || len(all) != 1 || recognized[0] != all[0] {
		t.Fatalf("device query paths diverged: err=%v", err)
	}
	device, err := st.GetDevice(mac)
	if err != nil || device == nil || *device != recognized[0] {
		t.Fatalf("single-device query diverged: err=%v", err)
	}
}
