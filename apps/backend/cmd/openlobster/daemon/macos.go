//go:build darwin

// # License
// See LICENSE in the root of the repository.
package daemon

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const plistLabel = "com.openlobster"

const plistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{ .Label }}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{ .BinaryPath }}</string>
		<string>serve</string>
	</array>
	<key>KeepAlive</key>
	<true/>
	<key>RunAtLoad</key>
	<true/>
	<key>WorkingDirectory</key>
	<string>{{ .WorkingDir }}</string>
	<key>StandardOutPath</key>
	<string>{{ .LogOut }}</string>
	<key>StandardErrorPath</key>
	<string>{{ .LogErr }}</string>
</dict>
</plist>
`

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist"), nil
}

func install(binaryPath, openlobsterHome string) error {
	logDir := filepath.Join(openlobsterHome, "logs")
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents directory: %w", err)
	}

	tmpl, err := template.New("plist").Parse(plistTmpl)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"Label":      plistLabel,
		"BinaryPath": binaryPath,
		"WorkingDir": openlobsterHome,
		"LogOut":     filepath.Join(logDir, "daemon.log"),
		"LogErr":     filepath.Join(logDir, "daemon.err"),
	}); err != nil {
		return err
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	fmt.Printf("  wrote %s\n", path)

	target := fmt.Sprintf("gui/%d", os.Getuid())

	out, err := exec.Command("launchctl", "bootstrap", target, path).CombinedOutput()
	if err != nil {
		// Service may already be loaded — attempt a graceful restart instead.
		out2, err2 := exec.Command("launchctl", "kickstart", "-k", target+"/"+plistLabel).CombinedOutput()
		if err2 != nil {
			fmt.Fprintf(os.Stderr, "launchctl: %s\n", bytes.TrimSpace(out))
			return fmt.Errorf("launchctl bootstrap: %w", err)
		}
		fmt.Printf("  service restarted: %s\n", bytes.TrimSpace(out2))
		return nil
	}

	fmt.Println("  daemon installed and started")
	return nil
}

func uninstall() error {
	path, err := plistPath()
	if err != nil {
		return err
	}

	target := fmt.Sprintf("gui/%d", os.Getuid())
	out, _ := exec.Command("launchctl", "bootout", target, path).CombinedOutput()
	if len(out) > 0 {
		fmt.Printf("  launchctl: %s\n", bytes.TrimSpace(out))
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}

	fmt.Println("  daemon stopped and uninstalled")
	return nil
}

func status() error {
	out, err := exec.Command("launchctl", "list", plistLabel).CombinedOutput()
	if err != nil {
		fmt.Println("  daemon: not running (or not installed)")
		return nil
	}
	fmt.Print(string(out))
	return nil
}
