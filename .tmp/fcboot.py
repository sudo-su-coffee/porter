import os, subprocess, time

W = "/tmp/fc-manual"
os.makedirs(W, exist_ok=True)
REF = "/mnt/d/github/porter1/references"

inner = """set -e
ip tuntap add dev tap9 mode tap
ip addr add 172.16.9.1/30 dev tap9
ip link set dev tap9 up
rm -f SOCKDIR/api.sock
FIRECRACKER_BIN --api-sock SOCKDIR/api.sock --log-path SOCKDIR/fc.log --level Info &
FCPID=$!
echo $FCPID > SOCKDIR/pid
for i in $(seq 1 50); do [ -S SOCKDIR/api.sock ] && break; sleep 0.1; done
[ -S SOCKDIR/api.sock ] || { echo NOSOCK; kill $FCPID; exit 1; }
C="curl -s --unix-socket SOCKDIR/api.sock -X PUT -H Content-Type:application/json -d"
$C '{"log_path":"SOCKDIR/fc.log","level":"Info","show_level":true,"show_log_origin":true}' http://localhost/logger
$C '{"kernel_image_path":"REFDIR/vmlinux","boot_args":"console=ttyS0 reboot=k panic=1 pci=off nomodules ro root=/dev/vda ip=172.16.9.2::172.16.9.1:255.255.255.252::eth0:off"}' http://localhost/boot-source
$C '{"drive_id":"rootfs","path_on_host":"REFDIR/rootfs.ext4","is_root_device":true,"is_read_only":false}' http://localhost/drives/rootfs
$C '{"iface_id":"eth0","guest_mac":"06:11:22:33:44:55","host_dev_name":"tap9"}' http://localhost/network-interfaces/eth0
$C '{"vcpu_count":1,"mem_size_mib":256}' http://localhost/machine-config
$C '{"action_type":"InstanceStart"}' http://localhost/actions
echo BOOTED
sleep 10
kill $FCPID 2>/dev/null || true
exit 0
""".replace("SOCKDIR", W).replace("REFDIR", REF).replace("FIRECRACKER_BIN", "/usr/local/bin/firecracker")

p = os.path.join(W, "inner.sh")
open(p, "w", newline="\n").write(inner)
os.chmod(p, 0o755)
r = subprocess.run(["unshare", "-Urn", "bash", p], capture_output=True, text=True, timeout=90)
print("RC:", r.returncode)
print("OUT:", r.stdout[-500:])
print("ERR:", r.stderr[-500:])
print("=== fc.log tail ===")
try:
    log = open(os.path.join(W, "fc.log"), errors="replace").read()
    print(log[-1500:])
except Exception as e:
    print("no log:", e)
