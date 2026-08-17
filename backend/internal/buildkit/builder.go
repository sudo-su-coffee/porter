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
	Bin  string
	Addr string
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
	cmd := exec.CommandContext(ctx, "mkfs.ext4", "-F", "-d", root, rootfsPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return Result{}, fmt.Errorf("mkfs.ext4: %w: %s", err, strings.TrimSpace(string(out)))
	}
	cfg, err := readOCIConfig(ociPath)
	if err != nil {
		return Result{RootfsPath: rootfsPath}, err
	}
	return Result{OCIPath: ociPath, RootfsPath: rootfsPath, Entrypoint: cfg.Config.Entrypoint, Cmd: cfg.Config.Cmd, Env: cfg.Config.Env, WorkingDir: cfg.Config.WorkingDir}, nil
}

func unpackOCI(path, root string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	t, err := tar.NewReader(f).Next()
	if err != nil {
		return fmt.Errorf("oci index: %w", err)
	}
	if t.Name != "index.json" {
		return fmt.Errorf("oci archive missing index.json")
	}
	var index ociIndex
	b, err := io.ReadAll(tar.NewReader(f))
	_ = b
	// The first tar reader was advanced; reopen to read index deterministically.
	f.Close()
	f, err = os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	var indexBytes []byte
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if h.Name == "index.json" {
			indexBytes, err = io.ReadAll(tr)
			break
		}
	}
	if err != nil {
		return err
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
