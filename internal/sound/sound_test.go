package sound

import (
	"os"
	"testing"
)

func TestEmbeddedSounds(t *testing.T) {
	if len(blip) == 0 {
		t.Fatal("embedded blip sound is empty")
	}
	if len(done) == 0 {
		t.Fatal("embedded done sound is empty")
	}
}

func TestPlayerCommand(t *testing.T) {
	command := playerCommand("test sound.wav")
	if command == nil && runtimePlayerExpected() {
		t.Fatal("native audio player command is unavailable")
	}
}

func runtimePlayerExpected() bool {
	_, err := os.Stat("/usr/bin/afplay")
	return err == nil
}
