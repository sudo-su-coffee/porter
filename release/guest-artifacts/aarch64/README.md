# Porter aarch64 guest artifacts

Upload a compatible aarch64 guest kernel and rootfs into this folder using the
same filenames:

```text
release/guest-artifacts/aarch64/vmlinux
release/guest-artifacts/aarch64/rootfs.ext4
```

Do not commit placeholder files. The release workflow verifies both files and
calculates SHA-256 sidecars automatically.
