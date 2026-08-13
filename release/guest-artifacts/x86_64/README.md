# Porter x86_64 guest artifacts

For small or Git-LFS-managed artifacts, upload the two real guest files for the
x86_64 Porter release into this folder:

```text
release/guest-artifacts/x86_64/vmlinux
release/guest-artifacts/x86_64/rootfs.ext4
```

Do not commit placeholder files. `vmlinux` must be a compatible x86_64 Linux
guest kernel and `rootfs.ext4` must be a bootable ext4 filesystem with an init
system and the guest networking requirements. The GitHub Actions release
workflow checks that both files are non-empty, calculates their SHA-256 values,
and includes them in the generated base-image archive. The Firecracker VMM
binary remains a separate official GitHub release dependency installed and
verified by the Linux installer.

For files around 50 MB, the preferred path is a separate GitHub Release named
`base-images-v1.0.0-beta-dev` with two assets named exactly `vmlinux` and
`rootfs.ext4`. The workflow downloads those assets when the repository folder is
empty and calculates their SHA-256 values automatically; no SHA input is needed.
