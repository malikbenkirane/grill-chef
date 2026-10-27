package daemoninfo

import "testing"

func TestWriteThenReadMarker(t *testing.T) {
	path := t.TempDir() + "/daemon.json"
	if err := Write(path, Info{PID: 42, Port: 8080}); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.PID != 42 || got.Port != 8080 {
		t.Fatalf("lossy marker: %+v", got)
	}
}

func TestReadMissingMarkerIsError(t *testing.T) {
	if _, err := Read(t.TempDir() + "/nope.json"); err == nil {
		t.Fatal("missing marker must be an error: recovery treats no marker as daemon-down")
	}
}

func TestWriteIsAtomicOnSamePath(t *testing.T) {
	path := t.TempDir() + "/daemon.json"
	if err := Write(path, Info{PID: 1, Port: 2}); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, Info{PID: 3, Port: 4}); err != nil {
		t.Fatal(err)
	}
	got, _ := Read(path)
	if got.PID != 3 || got.Port != 4 {
		t.Fatal("second write must replace first atomically (no torn reads)")
	}
}
