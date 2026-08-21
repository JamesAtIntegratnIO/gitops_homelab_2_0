# Game Day: proving the cluster heals

> Everything in the self-healing work is a hypothesis until a node actually goes
> away. This is the procedure that turns it into a measurement.

Run it quarterly, and once after any change to scheduling, priorities, replica
counts or the Talos machine config. Budget an hour; the disruptive part is about
fifteen minutes.

## Before you start

Pick a quiet window. Announce it if anyone else uses the media stack — the
tenant vcluster is in scope and its apps will blip.

Confirm the cluster is actually healthy first, or you will be measuring the
wrong thing:

```bash
kubectl get nodes
kubectl -n argocd get applications -o custom-columns='NAME:.metadata.name,SYNC:.status.sync.status,HEALTH:.status.health.status' | grep -v 'Synced *Healthy'
kubectl get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded
```

All nodes `Ready`, no app off `Synced/Healthy`, no pods outside
`Running`/`Succeeded`. If something is already broken, fix it or postpone.

Record the starting state — you are going to compare against it:

```bash
kubectl get pods -A -o wide > /tmp/gameday-before.txt
for n in $(kubectl get nodes -o name); do echo "== $n"; kubectl describe "$n" | grep -A6 'Allocated resources'; done
```

## Check 1: does N+1 still hold?

This is arithmetic, not an experiment, and it is the check most likely to drift
as workloads are added. Total CPU requests must fit on any two of the three
nodes.

```bash
kubectl get pods -A -o json | jq -r '
  def cpu: if .==null then 0
           elif (type=="string" and test("m$")) then (.[:-1]|tonumber)
           elif (type=="string" and test("n$")) then ((.[:-1]|tonumber)/1000000)
           else ((tostring|tonumber)*1000) end;
  [.items[] | select(.status.phase=="Running")
   | [.spec.containers[].resources.requests.cpu | cpu] | add] | add
  | "total CPU requests: \(.|floor)m"'

kubectl get nodes -o json | jq -r '
  [.items[] | {n: .metadata.name, a: (.status.allocatable.cpu | if test("m$") then (.[:-1]|tonumber) else (tonumber*1000) end)}]
  | (map(.a)|add) as $all | (map(.a)|max) as $biggest
  | "two-node capacity:   \($all - $biggest)m"'
```

Second number minus first is your headroom after losing the largest node. It
should be positive with room to spare. If it has gone negative, stop here and
right-size before doing anything disruptive — otherwise you are about to
demonstrate a problem you already know about.

## Check 2: reboot a node

Start with a node that is *not* currently hosting the ArgoCD application
controller that holds the lease, so you are testing failover rather than
watching ArgoCD restart itself first.

```bash
kubectl get pods -A -o wide | grep -E 'nginx-gateway-nginx|kyverno-admission|coredns'
```

Pick your victim, then — in a second terminal, start the clock:

```bash
kubectl get pods -A -w | grep -E 'Pending|Terminating|ContainerCreating'
```

```bash
talosctl -n 10.0.4.10X reboot
```

Watch for, and write down the time to each:

| What | Expected | Why it matters |
|---|---|---|
| Node goes `NotReady` | ~10-40s | kubelet lease expiry |
| Gateway still serving | **no gap** | the second data-plane replica is on another node |
| Pods start moving | ~60s after NotReady | the toleration change; was 300s |
| Any pod stuck `Pending` | **none** | this is the N+1 check, live |
| Node back `Ready` | 2-4 min | Talos boots from disk |
| All apps `Synced/Healthy` | within ~5 min of Ready | |

The gateway check is worth doing properly rather than by eye:

```bash
while true; do
  printf '%s ' "$(date +%T)"
  curl -s -o /dev/null -w '%{http_code}\n' https://argocd.cluster.integratn.tech/healthz || echo FAIL
  sleep 1
done
```

Any run of non-200s is a real outage and the number to care about.

## Check 3: what did *not* come back

The failure mode that matters is not the reboot, it is the recovery being
incomplete in a way nobody notices for weeks.

```bash
kubectl get pods -A -o wide > /tmp/gameday-after.txt
diff <(awk '{print $1, $2}' /tmp/gameday-before.txt | sort) \
     <(awk '{print $1, $2}' /tmp/gameday-after.txt  | sort)

kubectl -n argocd get applications -o custom-columns='NAME:.metadata.name,SYNC:.status.sync.status,HEALTH:.status.health.status' | grep -v 'Synced *Healthy'
kubectl get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded
kubectl get events -A --field-selector type=Warning --sort-by=.lastTimestamp | tail -30
```

Then check the two things that only show up after a node loss:

```bash
# Did anything land unbalanced and stay there? The descheduler runs every 30
# minutes; give it a pass before judging.
for n in $(kubectl get nodes -o name); do echo "== $n"; kubectl describe "$n" | grep -A6 'Allocated resources'; done

# Did the terminal-pod cleanup policy do its job? (hourly, so allow one cycle)
kubectl get pods -A --field-selector=status.phase=Failed
```

## Check 4: the DNS dependency

The vcluster restart storms trace to host DNS stalling, not to anything the
reboot does — but a reboot is a good moment to confirm the blast radius is
smaller than it was. During and after the reboot:

```bash
kubectl -n kube-system logs -l k8s-app=kube-dns --tail=100 | grep -c 'i/o timeout'
kubectl -n vcluster-media get pods | grep -v Running
```

A handful of timeouts while the node is down is expected. Hundreds, or
in-vcluster pods restarting, means the CoreDNS PDB is not enough on its own and
the Talos-side work in
[matchbox/talos-machineconfigs/commands.md](../matchbox/talos-machineconfigs/commands.md)
should move up the list.

## Record the result

Append a row to the table below and commit it. The trend is the point — a single
run tells you much less than four runs do.

| Date | Node | Gateway gap | Longest Pending | Time to all-Healthy | Notes |
|---|---|---|---|---|---|
| _(first run pending)_ | | | | | |

## What this does not test

Be honest about the gaps rather than reading more into a green run than it
earned:

- **Two nodes at once.** Nothing here survives that: etcd loses quorum. That is
  a rebuild, and the rebuild path is untested (see below).
- **Rebuild from git.** Blocked today by two known issues: `flake.nix` sources
  a gitignored `secrets.env` unguarded, and `terraform/cluster/main.tf` reads a
  gitignored `dockerconfig.json` at plan time. Both break a fresh clone. Worth
  fixing before anyone needs the DR path in anger.
- **etcd loss.** There are no off-cluster etcd snapshots yet; the Talos API
  access that enables them is committed but not applied.
- **NFS server loss.** Every PV lives on it. `hack/restart-nfs-pods.sh` recovers
  stale mounts after it comes back; nothing covers it being gone.
