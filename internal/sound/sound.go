package sound

import (
	_ "embed"
	"os"
	"os/exec"
	"runtime"
)

//go:embed assets/blip.wav
var blip []byte

//go:embed assets/done.mp3
var done []byte

func Blip() { play(blip, ".wav") }

func Done() { play(done, ".mp3") }

func play(data []byte, extension string) {
	go func() {
		file, err := os.CreateTemp("", "ibot-sound-*"+extension)
		if err != nil {
			return
		}
		path := file.Name()
		defer os.Remove(path)
		if _, err := file.Write(data); err != nil {
			file.Close()
			return
		}
		if err := file.Close(); err != nil {
			return
		}
		command := playerCommand(path)
		if command != nil {
			_ = command.Run()
		}
	}()
}

func playerCommand(path string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("afplay", path)
	case "windows":
		return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
			"$player = New-Object -ComObject WMPlayer.OCX; $player.URL = $args[0]; $player.controls.play(); while ($player.playState -ne 1) { Start-Sleep -Milliseconds 50 }", path)
	default:
		for _, player := range []struct {
			name string
			args []string
		}{
			{"ffplay", []string{"-nodisp", "-autoexit", "-loglevel", "quiet", path}},
			{"mpv", []string{"--no-video", "--really-quiet", path}},
			{"paplay", []string{path}},
		} {
			if _, err := exec.LookPath(player.name); err == nil {
				return exec.Command(player.name, player.args...)
			}
		}
		return nil
	}
}
