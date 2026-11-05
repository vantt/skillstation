package app

import (
	"net"
	"path/filepath"
	"testing"
)

func TestInstalledHubServiceNeedsTheUnitFileOfThisSystem(t *testing.T) {
	t.Parallel()
	exists := func(want string) func(string) bool {
		return func(path string) bool { return path == filepath.FromSlash(want) }
	}
	linux := hubServiceEnv{goos: "linux", home: "/h", exists: exists("/h/.config/systemd/user/skillstation-server.service")}
	if service, ok := installedHubService(linux); !ok || service.Command != "systemctl --user restart skillstation-server.service" {
		t.Fatalf("linux: %+v %v", service, ok)
	}
	mac := hubServiceEnv{goos: "darwin", home: "/Users/me", uid: "501", exists: exists("/Users/me/Library/LaunchAgents/dev.skillstation.server.plist")}
	if service, ok := installedHubService(mac); !ok || service.Command != "launchctl kickstart -k gui/501/dev.skillstation.server" {
		t.Fatalf("macOS: %+v %v", service, ok)
	}
	for name, env := range map[string]hubServiceEnv{
		"no unit file":   {goos: "linux", home: "/h", exists: func(string) bool { return false }},
		"other system":   {goos: "windows", home: `C:\h`, exists: func(string) bool { return true }},
		"no home folder": {goos: "linux", exists: func(string) bool { return true }},
	} {
		if service, ok := installedHubService(env); ok {
			t.Errorf("%s: found %+v", name, service)
		}
	}
}

func TestDoctorFlagsAHubOnThisMachineThatWasNotRestartedAfterTheUpdate(t *testing.T) {
	t.Parallel()
	mine := func() []net.IP { return []net.IP{net.ParseIP("100.64.0.9")} }
	installed := HubService{Command: "systemctl --user restart skillstation-server.service", argv: []string{"systemctl", "--user", "restart", SystemdServiceUnit}}
	stale := HubProbe{Status: &HubStatus{Version: "0.0.1"}}
	current := HubProbe{Status: &HubStatus{Version: CurrentHubStatus().Version}}
	run := func(hub string, probe HubProbe, find func() (HubService, bool)) RemoteCheck {
		check := RemoteCheck{ID: "hub_version", Status: CheckOK, Detail: "ok"}
		RemoteDoctorService{LocalService: find, LocalIPs: mine}.flagStaleLocalHub(hub, probe, &check)
		return check
	}
	withService := func() (HubService, bool) { return installed, true }
	noService := func() (HubService, bool) { return HubService{}, false }

	for _, hub := range []string{"http://100.64.0.9:7420", "http://127.0.0.1:7420"} {
		check := run(hub, stale, withService)
		if check.Status != CheckFailed || check.restart == nil || check.manual != installed.Command {
			t.Errorf("%s: a stale hub of this machine was not flagged with its restart: %+v", hub, check)
		}
	}
	if check := run("http://100.64.0.9:7420", stale, noService); check.Status != CheckFailed || check.restart != nil || check.Fix == "" {
		t.Errorf("without a service there is advice but no fix: %+v", check)
	}
	if check := run("http://100.64.0.10:7420", stale, withService); check.Status != CheckOK {
		t.Errorf("a hub on another machine is not this machine's to restart: %+v", check)
	}
	if check := run("http://100.64.0.9:7420", current, withService); check.Status != CheckOK {
		t.Errorf("a hub on the current version was flagged: %+v", check)
	}
	finding := remoteFinding("http://100.64.0.9:7420", run("http://100.64.0.9:7420", stale, withService))
	if finding.Severity != SeverityProblem || finding.Fix == nil || finding.Manual != installed.Command {
		t.Errorf("finding: %+v", finding)
	}
}
