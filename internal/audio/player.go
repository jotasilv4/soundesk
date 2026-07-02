package audio

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var (
	activeCmds   = make(map[string]*exec.Cmd)
	activeCmdsMu sync.Mutex
)

// Play plays the sound file on the server host using the appropriate CLI player
func Play(filePath string) error {
	// Auto-stop any active playback to avoid overlapping sounds
	StopAll()

	ext := strings.ToLower(filepath.Ext(filePath))
	var cmd *exec.Cmd

	// Get absolute path to avoid directory context issues
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	if runtime.GOOS == "windows" {
		winPath := filepath.Clean(absPath)
		
		switch ext {
		case ".wav":
			// Windows native PowerShell play for WAV
			psCmd := fmt.Sprintf("(New-Object Media.SoundPlayer '%s').PlaySync()", winPath)
			cmd = exec.Command("powershell", "-Command", psCmd)
		default:
			// Windows native COM object player for MP3 and other formats
			psCmd := fmt.Sprintf("$p = New-Object -ComObject WMPlayer.OCX; $p.URL = '%s'; $p.controls.play(); while($p.playState -ne 1) { Start-Sleep -Milliseconds 100 }", winPath)
			cmd = exec.Command("powershell", "-Command", psCmd)
		}
	} else {
		// Linux/Unix
		switch ext {
		case ".mp3":
			cmd = exec.Command("mpg123", filePath)
		case ".wav":
			cmd = exec.Command("aplay", filePath)
		default:
			if ext == ".ogg" || ext == ".m4a" || ext == ".flac" {
				cmd = exec.Command("mpg123", filePath)
			} else {
				cmd = exec.Command("aplay", filePath)
			}
		}
	}

	playID := uuid.New().String()
	log.Printf("[AUDIO PLAYER] Starting playback of %s via %s (OS: %s) [ID: %s]", filePath, cmd.Path, runtime.GOOS, playID)

	activeCmdsMu.Lock()
	activeCmds[playID] = cmd
	activeCmdsMu.Unlock()

	go func() {
		defer func() {
			activeCmdsMu.Lock()
			delete(activeCmds, playID)
			activeCmdsMu.Unlock()
		}()

		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[AUDIO PLAYER] Playback failed/warned for %s [ID: %s]: %v. Output: %s", filePath, playID, err, string(output))
		} else {
			log.Printf("[AUDIO PLAYER] Playback completed successfully for %s [ID: %s]", filePath, playID)
		}
	}()

	return nil
}

// StopAll terminates all active CLI audio processes
func StopAll() {
	activeCmdsMu.Lock()
	defer activeCmdsMu.Unlock()

	if len(activeCmds) == 0 {
		return
	}

	log.Printf("[AUDIO PLAYER] Stopping all active playbacks (%d active commands)", len(activeCmds))
	for id, cmd := range activeCmds {
		if cmd != nil && cmd.Process != nil {
			log.Printf("[AUDIO PLAYER] Killing command process [ID: %s]", id)
			err := cmd.Process.Kill()
			if err != nil {
				log.Printf("[AUDIO PLAYER] Error killing process [ID: %s]: %v", id, err)
			}
		}
	}
	activeCmds = make(map[string]*exec.Cmd)
}
