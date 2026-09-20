// Package buildkit builds Dockerfiles through BuildKit and converts the
// resulting OCI image layout into a Firecracker-compatible ext4 rootfs.
package buildkit

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Builder struct {
	Bin         string
	Addr        string
	NixpacksBin string
	RailpackBin string
}

type Result struct {
	OCIPath     string
	RootfsPath  string
	Entrypoint  []string
	Cmd         []string
	Env         []string
	WorkingDir  string
	ImageDigest string
}

type ociIndex struct {
	Manifests []struct {
		Digest string `json:"digest"`
	} `json:"manifests"`
}
type ociManifest struct {
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		Digest    string `json:"digest"`
		MediaType string `json:"mediaType"`
	} `json:"layers"`
}
type imageConfig struct {
	Config struct {
		Entrypoint []string `json:"Entrypoint"`
		Cmd        []string `json:"Cmd"`
		Env        []string `json:"Env"`
		WorkingDir string   `json:"WorkingDir"`
	} `json:"config"`
}

// BuildWithPlan executes the detected engine. Dockerfile uses BuildKit;
// nixpacks/railpack shell out to their binaries (plan → OCI); static packs a
// publish dir; buildpacks needs a runner. Missing runners return an explicit
// error guiding to Dockerfile/prebuilt OCI (never fake an image).
func (b Builder) BuildWithPlan(ctx context.Context, p Plan) error {
	switch p.Engine {
	case EngineDockerfile:
		if p.Dockerfile == "" {
			return fmt.Errorf("buildkit: dockerfile engine but no Dockerfile in %s", p.ContextDir)
		}
		return b.Build(ctx, p.ContextDir, p.Dockerfile, p.OutputOCI)
	case EngineNixpacks:
		return runPack(ctx, firstNonEmpty(b.NixpacksBin, "nixpacks"), []string{"build", "--format", "oci", p.ContextDir, "--out", p.OutputOCI}, p.OutputOCI)
	case EngineRailpack:
		return runPack(ctx, firstNonEmpty(b.RailpackBin, "railpack"), []string{"build", "--format", "oci", p.ContextDir, "--out", p.OutputOCI}, p.OutputOCI)
	case EngineStatic:
		return fmt.Errorf("buildkit: static engine needs a publish dir build first (dist/build → nginx OCI); provide a Dockerfile or prebuilt OCI tar at %s", p.OutputOCI)
	default:
		return fmt.Errorf("buildkit: buildpacks runner not installed for %s; provide a Dockerfile or prebuilt OCI tar at %s", p.ContextDir, p.OutputOCI)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// runPack runs an external pack binary; a usable prebuilt OCI tar after the
// run still counts as success (daemon flaked but artifact exists).
func runPack(ctx context.Context, bin string, args []string, outputOCI string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		if validOCITar(outputOCI) {
			return nil
		}
		return fmt.Errorf("buildkit: %s: %w: %s", bin, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureOCI builds via BuildKit, falling back to a prebuilt OCI tar when the
// daemon/binary is unavailable (Coolify "Docker Image / prebuilt" parity).
// If outputOCI already holds a valid OCI layout (index.json present) it is
// reused as-is so air-gapped / CI-built images never rebuild.
func (b Builder) EnsureOCI(ctx context.Context, contextDir, dockerfile, outputOCI string) error {
	if outputOCI != "" && validOCITar(outputOCI) {
		return nil
	}
	if err := b.Build(ctx, contextDir, dockerfile, outputOCI); err != nil {
		if outputOCI != "" && validOCITar(outputOCI) {
			return nil // daemon flaked but a usable artifact exists
		}
		return err
	}
	return nil
}

// validOCITar reports whether path looks like a BuildKit OCI tar (has index.json).
func validOCITar(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			return false
		}
		if e != nil {
			return false
		}
		if h.Name == "index.json" {
			return true
		}
	}
}

func (b Builder) Build(ctx context.Context, contextDir, dockerfile, outputOCI string) error {
	if b.Bin == "" {
		b.Bin = "buildctl"
	}
	if b.Addr == "" {
		b.Addr = "unix:///run/buildkit/buildkitd.sock"
	}
	if contextDir == "" || dockerfile == "" || outputOCI == "" {
		return fmt.Errorf("buildkit: context, dockerfile, and output are required")
	}
	if err := os.MkdirAll(filepath.Dir(outputOCI), 0o750); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, b.Bin, "--addr", b.Addr, "build", "--frontend", "dockerfile.v0", "--local", "context="+contextDir, "--local", "dockerfile="+filepath.Dir(dockerfile), "--opt", "filename="+filepath.Base(dockerfile), "--output", "type=oci,dest="+outputOCI)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("buildkit build: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ConvertOCIToExt4(ctx context.Context, ociPath, rootfsPath string, sizeMiB int) (Result, error) {
	if sizeMiB < 64 {
		sizeMiB = 512
	}
	root, err := os.MkdirTemp("", "porter-oci-root-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(root)
	if err := unpackOCI(ociPath, root); err != nil {
		return Result{}, err
	}
	cfg, cfgErr := readOCIConfig(ociPath)
	if cfgErr == nil {
		// Images without ENTRYPOINT/CMD (plain OS bases) keep their own /sbin/init.
		if script, err := BuildInitScript(cfg.Config.Entrypoint, cfg.Config.Cmd, cfg.Config.Env, cfg.Config.WorkingDir); err == nil {
			if err := InstallInit(root, script); err != nil {
				return Result{}, fmt.Errorf("install init shim: %w", err)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(rootfsPath), 0o750); err != nil {
		return Result{}, err
	}
	f, err := os.OpenFile(rootfsPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o640)
	if err != nil {
		return Result{}, err
	}
	if err := f.Truncate(int64(sizeMiB) * 1024 * 1024); err != nil {
		f.Close()
		return Result{}, err
	}
	f.Close()
	if _, err := exec.LookPath("mkfs.ext4"); err != nil {
		return Result{}, fmt.Errorf("mkfs.ext4 not found: rootfs formatting requires Linux with e2fsprogs (unpack to %s already staged, image build must run on Linux)", root)
	}
	cmd := exec.CommandContext(ctx, "mkfs.ext4", "-F", "-d", root, rootfsPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return Result{}, fmt.Errorf("mkfs.ext4: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if cfgErr != nil {
		return Result{RootfsPath: rootfsPath}, cfgErr
	}
	return Result{OCIPath: ociPath, RootfsPath: rootfsPath, Entrypoint: cfg.Config.Entrypoint, Cmd: cfg.Config.Cmd, Env: cfg.Config.Env, WorkingDir: cfg.Config.WorkingDir}, nil
}

func unpackOCI(path, root string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// Scan for index.json anywhere: real buildx tars list blobs/ first, so no
	// first-entry assumption (that strict check rejected valid OCI layouts).
	var index ociIndex
	var indexBytes []byte
	tr := tar.NewReader(f)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if h.Name == "index.json" || h.Name == "./index.json" {
			indexBytes, err = io.ReadAll(tr)
			break
		}
	}
	if err != nil {
		return err
	}
	if len(indexBytes) == 0 {
		return fmt.Errorf("oci archive missing index.json")
	}
	if err := json.Unmarshal(indexBytes, &index); err != nil || len(index.Manifests) == 0 {
		return fmt.Errorf("invalid OCI index")
	}
	manifestBytes, err := readBlob(path, index.Manifests[0].Digest)
	if err != nil {
		return err
	}
	var manifest ociManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return err
	}
	for _, layer := range manifest.Layers {
		data, err := readBlob(path, layer.Digest)
		if err != nil {
			return err
		}
		if err := applyLayer(data, root); err != nil {
			return err
		}
	}
	return nil
}

func readOCIConfig(path string) (imageConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return imageConfig{}, err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	var idx []byte
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return imageConfig{}, e
		}
		if h.Name == "index.json" {
			idx, err = io.ReadAll(tr)
			break
		}
	}
	if err != nil {
		return imageConfig{}, err
	}
	var i ociIndex
	if err := json.Unmarshal(idx, &i); err != nil {
		return imageConfig{}, err
	}
	m, err := readBlob(path, i.Manifests[0].Digest)
	if err != nil {
		return imageConfig{}, err
	}
	var man ociManifest
	if err := json.Unmarshal(m, &man); err != nil {
		return imageConfig{}, err
	}
	c, err := readBlob(path, man.Config.Digest)
	if err != nil {
		return imageConfig{}, err
	}
	var cfg imageConfig
	return cfg, json.Unmarshal(c, &cfg)
}

func readBlob(path, digest string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	want := "blobs/" + strings.Replace(digest, ":", "/", 1)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		if h.Name == want {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("OCI blob %s not found", digest)
}

func applyLayer(data []byte, root string) error {
	var r io.Reader = strings.NewReader(string(data))
	if gz, err := gzip.NewReader(strings.NewReader(string(data))); err == nil {
		defer gz.Close()
		r = gz
	}
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		rel := filepath.Clean(h.Name)
		if rel == "." || strings.HasPrefix(rel, "../") {
			continue
		}
		dst := filepath.Join(root, rel)
		base := filepath.Base(rel)
		if strings.HasPrefix(base, ".wh.") {
			if base == ".wh..wh..opq" {
				entries, _ := os.ReadDir(filepath.Dir(dst))
				for _, e := range entries {
					_ = os.RemoveAll(filepath.Join(filepath.Dir(dst), e.Name()))
				}
			} else {
				_ = os.RemoveAll(filepath.Join(filepath.Dir(dst), strings.TrimPrefix(base, ".wh.")))
			}
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dst, os.FileMode(h.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			_ = os.RemoveAll(dst)
			if err := os.Symlink(h.Linkname, dst); err != nil {
				return err
			}
		}
	}
}
