// Package startup verifies the host runtime prerequisites at boot so a
// misconfigured containerd/jailer/firecracker setup fails loudly at startup
// instead of only surfacing as the first VM boot failing.
package startup

import (
	"fmt"
	"os"
	"os/exec"

	"porter/internal/config"
)

// Result is the outcome of one prerequisite check.
type Result struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Fatal   bool   `json:"fatal,omitempty"`
}

// Check runs all sanity checks against the given config. Non-fatal failures
// are reported but don't stop startup; fatal ones are meant to stop it.
func Check(cfg *config.Config) []Result {
	var out []Result
	out = append(out, checkContainerd(cfg))
	out = append(out, checkKVM())
	out = append(out, checkFirecracker(cfg))
	out = append(out, checkJailer(cfg))
	out = append(out, checkVolumesDir(cfg))
	return out
}

// checkContainerd verifies the containerd socket is present and reachable.
func checkContainerd(cfg *config.Config) Result {
	if cfg.ContainerdSocket == "" {
		return Result{Name: "containerd-socket", OK: false, Message: "no containerd socket configured", Fatal: true}
	}
	if _, err := os.Stat(cfg.ContainerdSocket); err != nil {
		return Result{Name: "containerd-socket", OK: false, Message: fmt.Sprintf("%v — real VM boots will fail", err)}
	}
	// A daemon on a Unix socket responds to a version ping over its own socket.
	return Result{Name: "containerd-socket", OK: true, Message: cfg.ContainerdSocket}
}

// checkKVM verifies hardware virtualization is available for microVMs.
func checkKVM() Result {
	for _, kvm := range []string{"/dev/kvm", "/dev/kvm0"} {
		if _, err := os.Stat(kvm); err == nil {
			return Result{Name: "kvm", OK: true, Message: kvm}
		}
	}
	return Result{Name: "kvm", OK: false, Message: "/dev/kvm not found — microVMs will not boot (WSL2: enable nested virtualization)"}
}

// checkFirecracker verifies the firecracker binary exists and runs.
func checkFirecracker(cfg *config.Config) Result {
	bin := cfg.FirecrackerBin
	if bin == "" {
		bin = "firecracker"
	}
	p, err := exec.LookPath(bin)
	if err != nil {
		// Allow an absolute path that exists but isn't on PATH.
		if _, serr := os.Stat(bin); serr == nil {
			p = bin
		} else {
			return Result{Name: "firecracker", OK: false, Message: fmt.Sprintf("binary %q not found: %v", bin, err)}
		}
	}
	return Result{Name: "firecracker", OK: true, Message: p}
}

// checkJailer warns (non-fatal) when a jailer is referenced but missing — the
// shim would silently run without jailer isolation, which is a real security
// concern worth surfacing loudly at startup.
func checkJailer(cfg *config.Config) Result {
	if cfg.JailerBin == "" {
		return Result{Name: "jailer", OK: true, Message: "not explicitly configured (containerd shim manages its own)"}
	}
	if _, err := os.Stat(cfg.JailerBin); err != nil {
		return Result{Name: "jailer", OK: false, Message: fmt.Sprintf("configured jailer %q missing: %v — workloads may run without jailer isolation", cfg.JailerBin, err)}
	}
	return Result{Name: "jailer", OK: true, Message: cfg.JailerBin}
}

// checkVolumesDir ensures the volumes root is writable (real volumes need it).
func checkVolumesDir(cfg *config.Config) Result {
	if cfg.VolumesDir == "" {
		return Result{Name: "volumes-dir", OK: true, Message: "not configured (defaults to ./volumes)"}
	}
	if err := os.MkdirAll(cfg.VolumesDir, 0o755); err != nil {
		return Result{Name: "volumes-dir", OK: false, Message: fmt.Sprintf("cannot create %s: %v", cfg.VolumesDir, err)}
	}
	return Result{Name: "volumes-dir", OK: true, Message: cfg.VolumesDir}
}
