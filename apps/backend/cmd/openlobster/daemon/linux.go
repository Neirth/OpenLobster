//go:build linux

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

const unitName = "openlobster.service"

const unitTmpl = `[Unit]
Description=OpenLobster Autonomous Messaging Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{ .BinaryPath }} serve
WorkingDirectory={{ .WorkingDir }}
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=openlobster
Environment=OLLAMA_AUTH=false

[Install]
WantedBy=default.target
`

func unitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", unitName), nil
}

func install(binaryPath, openlobsterHome string) error {
	path, err := unitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create systemd user directory: %w", err)
	}

	tmpl, err := template.New("unit").Parse(unitTmpl)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"BinaryPath": binaryPath,
		"WorkingDir": openlobsterHome,
	}); err != nil {
		return err
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	fmt.Printf("  wrote %s\n", path)

	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "systemctl daemon-reload: %s\n", bytes.TrimSpace(out))
		return fmt.Errorf("daemon-reload: %w", err)
	}

	if out, err := exec.Command("systemctl", "--user", "enable", unitName).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "systemctl enable: %s\n", bytes.TrimSpace(out))
		return fmt.Errorf("enable: %w", err)
	}

	if out, err := exec.Command("systemctl", "--user", "start", unitName).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "systemctl start: %s\n", bytes.TrimSpace(out))
		return fmt.Errorf("start: %w", err)
	}

	fmt.Println("  daemon installed and started")
	return nil
}

func uninstall() error {
	if out, err := exec.Command("systemctl", "--user", "stop", unitName).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "systemctl stop: %s\n", bytes.TrimSpace(out))
	}

	if out, err := exec.Command("systemctl", "--user", "disable", unitName).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "systemctl disable: %s\n", bytes.TrimSpace(out))
	}

	path, err := unitPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}

	exec.Command("systemctl", "--user", "daemon-reload").Run() //nolint:errcheck

	fmt.Println("  daemon stopped and uninstalled")
	return nil
}

func status() error {
	out, _ := exec.Command("systemctl", "--user", "status", unitName).CombinedOutput()
	if len(out) > 0 {
		fmt.Print(string(out))
	} else {
		fmt.Println("  daemon: not running (or not installed)")
	}
	return nil
}
