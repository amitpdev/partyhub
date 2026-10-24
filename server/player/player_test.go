package player

import (
	"reflect"
	"testing"
)

func TestCommandArgs(t *testing.T) {
	got := CommandArgs("/tmp/player.sock", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", 120)
	want := []string{
		"--no-video",
		"--force-window=no",
		"--really-quiet",
		"--input-ipc-server=/tmp/player.sock",
		"--volume=100",
		"--",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CommandArgs() = %#v, want %#v", got, want)
	}
}
