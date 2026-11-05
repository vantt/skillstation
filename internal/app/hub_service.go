package app

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The user service `skillstation serve --install-service` writes. The CLI
// installs it, and doctor and update restart it, so its name and place are
// defined here once.
const (
	SystemdServiceUnit  = "skillstation-server.service"
	LaunchdServiceLabel = "dev.skillstation.server"
)

// HubServiceUnitPath is where the service definition of goos for the user
// lives.
func HubServiceUnitPath(goos, home, configHome string) (string, bool) {
	switch goos {
	case "linux":
		if configHome == "" {
			configHome = filepath.Join(home, ".config")
		}
		return filepath.Join(configHome, "systemd", "user", SystemdServiceUnit), true
	case "darwin":
		return filepath.Join(home, "Library", "LaunchAgents", LaunchdServiceLabel+".plist"), true
	}
	return "", false
}

// HubService is the hub's user service on this machine.
type HubService struct {
	// Command is the restart command as a person would type it.
	Command string
	argv    []string
}

// hubServiceRunner runs one external command and returns its combined output.
var hubServiceRunner = func(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// hubServiceEnv is what finding the service reads from the machine.
type hubServiceEnv struct {
	goos, home, configHome, uid string
	exists                      func(path string) bool
}

func currentHubServiceEnv() hubServiceEnv {
	home, _ := os.UserHomeDir()
	uid := ""
	if current, err := user.Current(); err == nil {
		uid = current.Uid
	}
	return hubServiceEnv{goos: runtime.GOOS, home: home, configHome: os.Getenv("XDG_CONFIG_HOME"), uid: uid, exists: func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}}
}

// InstalledHubService finds the service `serve --install-service` wrote for
// this user, when there is one.
func InstalledHubService() (HubService, bool) {
	return installedHubService(currentHubServiceEnv())
}

func installedHubService(env hubServiceEnv) (HubService, bool) {
	if env.home == "" {
		return HubService{}, false
	}
	path, ok := HubServiceUnitPath(env.goos, env.home, env.configHome)
	if !ok || !env.exists(path) {
		return HubService{}, false
	}
	if env.goos == "darwin" {
		if env.uid == "" {
			return HubService{}, false
		}
		argv := []string{"launchctl", "kickstart", "-k", "gui/" + env.uid + "/" + LaunchdServiceLabel}
		return HubService{Command: strings.Join(argv, " "), argv: argv}, true
	}
	argv := []string{"systemctl", "--user", "restart", SystemdServiceUnit}
	return HubService{Command: strings.Join(argv, " "), argv: argv}, true
}

// Restart restarts the service. Agent sessions that were open reconnect on
// their next call.
func (service HubService) Restart(ctx context.Context) error {
	if len(service.argv) == 0 {
		return fmt.Errorf("no hub service")
	}
	if output, err := hubServiceRunner(ctx, service.argv[0], service.argv[1:]...); err != nil {
		if output == "" {
			output = err.Error()
		}
		return fmt.Errorf("`%s` failed: %s", service.Command, output)
	}
	return nil
}

// hubIsOnThisMachine reports whether hubURL names this machine: a loopback
// name, or an address one of its interfaces has.
func hubIsOnThisMachine(hubURL string, local func() []net.IP) bool {
	parsed, err := url.Parse(hubURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if isLoopbackName(host) {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, mine := range local() {
		if mine.Equal(ip) {
			return true
		}
	}
	return false
}

func interfaceIPs() []net.IP {
	addrs, _ := net.InterfaceAddrs()
	var ips []net.IP
	for _, addr := range addrs {
		if network, ok := addr.(*net.IPNet); ok {
			ips = append(ips, network.IP)
		}
	}
	return ips
}

// hubRestartWait is how long a restart waits for the hub to listen again: the
// service manager returns before the new process has opened its port.
const hubRestartWait = 15 * time.Second

// waitForListener reports whether something accepts connections at the host
// and port of hubURL within timeout.
func waitForListener(ctx context.Context, hubURL string, timeout time.Duration) bool {
	parsed, err := url.Parse(hubURL)
	if err != nil || parsed.Host == "" {
		return false
	}
	address := parsed.Host
	if parsed.Port() == "" {
		address = net.JoinHostPort(parsed.Hostname(), "80")
	}
	deadline := time.Now().Add(timeout)
	for {
		dialer := net.Dialer{Timeout: time.Second}
		if conn, err := dialer.DialContext(ctx, "tcp", address); err == nil {
			_ = conn.Close()
			return true
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(200 * time.Millisecond):
		}
	}
}
