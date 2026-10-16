"""Fake Prometheus, Alertmanager, Sonarr, Radarr, SABnzbd and Unraid for
preview/run.sh. Every answer is made up; nothing here talks to the cluster.

    python3 mock.py <state> <port>

States: busy (something wrong everywhere), quiet (all clear), fresh (no host
metrics yet), amdown (Alertmanager answers 503). Routing is by PromQL prefix,
so a widget whose query this does not know gets a single generic row.
"""
import json, sys, time
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse, parse_qs
now=time.time()
def vec(rows): return {"status":"success","data":{"resultType":"vector","result":[{"metric":m,"value":[now,str(v)]} for m,v in rows]}}
def iso(sec_ago): return time.strftime("%Y-%m-%dT%H:%M:%S.123Z", time.gmtime(now-sec_ago))
ALERTS=[
 {"labels":{"alertname":"KargoPromotionErrored","severity":"warning","namespace":"addons","stage":"external-secrets","promotion":"external-secrets.01m52gg2saeqfpr35wpkj06wn9.294f34b","component":"kargo"},
  "annotations":{"summary":"Kargo promotion errored (external-secrets)","description":"Promotion external-secrets.01m52gg2saeqfpr35wpkj06wn9.294f34b in addons could not run. This is an infrastructure fault rather than a rejected change.","kargo_url":"https://kargo.cluster.integratn.tech/project/addons/stage/external-secrets"},
  "startsAt":iso(7200),"generatorURL":"https://prometheus.cluster.integratn.tech/graph?g0.expr=kargo_stage_last_promotion_phase%7Bphase%3D%22Errored%22%7D+%3D%3D+1&g0.tab=1","status":{"state":"active","silencedBy":[],"inhibitedBy":[]}},
 {"labels":{"alertname":"PodLoggingStaleFileHandle","severity":"critical","exported_namespace":"authentik","exported_pod":"authentik-redis-master-0","container":"redis"},
  "annotations":{"summary":"authentik/authentik-redis-master-0 is logging stale file handle","description":"The pod's NFS volume went stale (Tower restarted). Delete the pod so it remounts."},
  "startsAt":iso(600),"generatorURL":"https://prometheus.cluster.integratn.tech/graph?g0.expr=x","status":{"state":"active","silencedBy":[],"inhibitedBy":[]}},
 {"labels":{"alertname":"ArgoCDAppUnhealthy","severity":"warning","name":"vcluster-media-sonarr","project":"default","health_status":"Degraded"},
  "annotations":{"summary":"ArgoCD app vcluster-media-sonarr is Degraded","description":"Application vcluster-media-sonarr (project default) has been Degraded for 15+ minutes. Its workloads are crashing, unschedulable, or absent.","logs_url":"https://grafana.cluster.integratn.tech/d/argocd-overview/argocd-overview"},
  "startsAt":iso(3600*26),"generatorURL":"https://prometheus.cluster.integratn.tech/graph?g0.expr=y","status":{"state":"active","silencedBy":[],"inhibitedBy":[]}},
 {"labels":{"alertname":"KubePodCrashLooping","severity":"warning","namespace":"mcp-system","pod":"grafana-mcp-7d9f8-abcde","container":"grafana-mcp"},
  "annotations":{"summary":"Pod is crash looping.","description":"Pod mcp-system/grafana-mcp-7d9f8-abcde (grafana-mcp) is in waiting state (reason: \"CrashLoopBackOff\").","runbook_url":"https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubepodcrashlooping"},
  "startsAt":iso(1800),"generatorURL":"https://prometheus.cluster.integratn.tech/graph?g0.expr=z","status":{"state":"active","silencedBy":[],"inhibitedBy":[]}},
 {"labels":{"alertname":"CPUThrottlingHigh","severity":"info","namespace":"monitoring","pod":"prom-0","container":"x"},"annotations":{"summary":"Processes experience elevated CPU throttling.","description":"40% throttling of CPU in namespace monitoring for container x in pod prom-0.","runbook_url":"https://runbooks.prometheus-operator.dev/runbooks/kubernetes/cputhrottlinghigh"},"startsAt":iso(500),"generatorURL":"https://prometheus.cluster.integratn.tech/graph?g0.expr=q","status":{"state":"active"}},
 {"labels":{"alertname":"KubePodCrashLooping","severity":"warning","namespace":"mcp-system","pod":"other-pod","container":"c"},"annotations":{"summary":"Pod is crash looping.","description":"Pod mcp-system/other-pod is crash looping."},"startsAt":iso(900),"generatorURL":"","status":{"state":"active"}},
 {"labels":{"alertname":"GitHubRunnerDown","severity":"warning","host":"pitdroid","runner":"specmarshal-7","job":"host-node"},"annotations":{"summary":"Runner specmarshal-7 on pitdroid is down","description":"The specmarshal-7 runner's service on pitdroid has not been running for 10 minutes, so jobs for it queue. `sudo systemctl restart github-runner-specmarshal-7`","action_url":"https://github.com/organizations/IntegratnIO/settings/actions/runners","action_title":"Runners on GitHub"},"startsAt":iso(1200),"generatorURL":"https://prometheus.cluster.integratn.tech/graph?g0.expr=r","status":{"state":"active"}},
 {"labels":{"alertname":"HostNotReporting","severity":"warning","host":"tower"},"annotations":{"summary":"tower is not reporting metrics","description":"Prometheus cannot scrape node_exporter on Tower (10.0.0.12:9100).","action_url":"http://10.0.0.12/Apps","action_title":"Unraid apps"},"startsAt":iso(1200),"generatorURL":"","status":{"state":"active"}},
 {"labels":{"alertname":"Watchdog","severity":"none"},"annotations":{"summary":"watchdog"},"startsAt":iso(99999),"generatorURL":"","status":{"state":"active"}},
]
MODE=sys.argv[1] if len(sys.argv)>1 else "busy"
class H(BaseHTTPRequestHandler):
    def log_message(self,*a): pass
    def send(self,obj,code=200):
        b=json.dumps(obj).encode(); self.send_response(code); self.send_header("Content-Type","application/json"); self.end_headers(); self.wfile.write(b)
    def do_POST(self):
        if urlparse(self.path).path.endswith("/graphql"):
            return self.send({"data":{"array":{"state":"STARTED","capacity":{"kilobytes":{"free":"9100000000","used":"31900000000","total":"41000000000"}}}}})
        return self.send({"error":"no"},404)
    def do_GET(self):
        u=urlparse(self.path); q=parse_qs(u.query).get("query",[""])[0]
        if u.path.startswith("/sonarr/"):
            img=[{"coverType":"poster","remoteUrl":""}]
            def ep(sid,title,slug,s_,e,t,off,has,mon=True): return {"seriesId":sid,"seasonNumber":s_,"episodeNumber":e,"title":t,"airDateUtc":iso(off),"hasFile":has,"monitored":mon,"series":{"title":title,"titleSlug":slug,"images":img}}
            return self.send([ep(1,"Crystal Lake","crystal-lake",1,n,f"Part {n}",86400,True) for n in range(2,7)]+[ep(2,"Law & Order: SVU","law-and-order-svu",28,2,"Ep 2",40000,False),ep(3,"Slow Horses","slow-horses",5,3,"Missing Pieces",-200000,False),ep(3,"Slow Horses","slow-horses",5,4,"Next",-800000,False)])
        if u.path.startswith("/radarr/"):
            def mv(t,slug,off,has): return {"title":t,"titleSlug":slug,"year":2025,"hasFile":has,"digitalRelease":iso(off),"images":[]}
            return self.send([mv("KPop Demon Hunters","kpop",900000,True),mv("Frankenstein","frankenstein",400000,True),mv("Backrooms","backrooms",300000,False),mv("Dune 3","dune-3",-2000000,False),{"title":"Cinema Only","titleSlug":"c","year":2026,"hasFile":False,"inCinemas":iso(-100000),"images":[]}])
        if u.path.startswith("/sab/"):
            mode=parse_qs(u.query).get("mode",[""])[0]
            if mode=="history": return self.send({"history":{"slots":[{"name":"some.show.s01e01.1080p","fail_message":"Unpacking failed, archive requires a password","completed":int(now-3600)},{"name":"old.fail","fail_message":"x","completed":int(now-900000)}]}})
            return self.send({"queue":{"paused":True,"status":"Paused","kbpersec":"0","timeleft":"0:00:00","slots":[{"filename":"law.and.order.svu.s28e02.1080p.web.h264-cakes","percentage":"0"}]}})
        quiet0 = MODE=="quiet"
        if u.path=="/ha/api/states":
            st=[{"entity_id":"alarm_control_panel.panel","state":"armed_home","attributes":{"friendly_name":"Panel"}},
                {"entity_id":"number.vaca_ha1588kk_wake_word_threshold","state":"6.0","attributes":{}},
                {"entity_id":"sensor.front_door_lock_battery","state":"8" if not quiet0 else "80","attributes":{"device_class":"battery","unit_of_measurement":"%","friendly_name":"Front door lock battery"}},
                {"entity_id":"sensor.garage_sensor_battery","state":"17","attributes":{"device_class":"battery","unit_of_measurement":"%","friendly_name":"Garage sensor battery"}},
                {"entity_id":"sensor.phone_battery","state":"unknown","attributes":{"device_class":"battery","unit_of_measurement":"%"}},
                {"entity_id":"light.kitchen","state":"on","attributes":{}}]+([] if quiet0 else [{"entity_id":"sensor.x%d"%i,"state":"unavailable","attributes":{}} for i in range(4)])
            return self.send(st)
        if u.path=="/jellyfin/Sessions":
            return self.send([] if quiet0 else [{"UserName":"james","Client":"Wholphin","PlayState":{"PlayMethod":"Transcode","IsPaused":False},"NowPlayingItem":{"Name":"Pilot","SeriesName":"Slow Horses"}},{"UserName":"kid","Client":"Android TV"}])
        if u.path=="/plex/status/sessions":
            return self.send({"MediaContainer":{"size":0}} if quiet0 else {"MediaContainer":{"size":2,"Metadata":[{"title":"Frankenstein","User":{"title":"james"},"Player":{"title":"Living Room TV","state":"playing"}},{"grandparentTitle":"Scrubs","title":"My Best Friend's Birthday","User":{"title":"kid"},"Player":{"title":"iPad","state":"paused"},"TranscodeSession":{"videoDecision":"transcode"}}]}})
        if u.path.startswith("/gh/repos/"):
            repo=u.path.split("/")[4]
            if repo=="nix_dots_2": return self.send({"message":"Not Found"},404)
            if quiet0 or repo=="kargo-observability": return self.send([])
            def pr(n,t,ref,draft,age): return {"number":n,"title":t,"html_url":f"https://github.com/JamesAtIntegratnIO/{repo}/pull/{n}","draft":draft,"created_at":iso(age).replace(".123",""),"user":{"login":"JamesAtIntegratnIO"},"head":{"ref":ref},"base":{"repo":{"name":repo}}}
            return self.send([pr(482,"feat(homepage): backups, certs, PRs, house, security, now playing","feat/glance-more",True,600),pr(481,"chore(deps): update glance to v0.8.8","kargo/promotion/homepage.01m5.abc",False,7200),pr(470,"fix(mcp-system): pin grafana-mcp","fix/mcp",False,90000)])
        if u.path=="/api/v2/alerts":
            if MODE=="amdown": return self.send({"err":"x"},503)
            return self.send([] if MODE=="quiet" else ALERTS)
        if u.path=="/api/v1/query_range":
            return self.send({"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[now-i*300,str(30+i%7)] for i in range(72)]}]}})
        if u.path!="/api/v1/query": return self.send({"error":"no"},404)
        quiet = MODE=="quiet"
        if q.startswith("count(") and "kube_pod_container_status_waiting_reason" in q: return self.send(vec([({},0 if quiet else 3)]))
        if "ALERTS" in q: return self.send(vec([({},1)]))
        if q.startswith("count(argocd_app_info{health_status"): return self.send(vec([({},0 if quiet else 1)]))
        if q.startswith("count(kargo_stage"): return self.send(vec([({},0 if quiet else 1)]))
        if "kube_node_status_condition" in q: return self.send(vec([({},3)]))
        if q.startswith("count(kube_node_info)"): return self.send(vec([({},3)]))
        if q.startswith("argocd_app_info"):
            return self.send(vec([] if quiet else [({"name":"vcluster-media-sonarr","namespace":"argocd","project":"default","dest_namespace":"vcluster-media","health_status":"Degraded","sync_status":"Synced"},1),({"name":"kyverno","namespace":"argocd","project":"platform-services","dest_namespace":"kyverno","health_status":"Healthy","sync_status":"OutOfSync"},1)]))
        if q.startswith("kargo_stage_last"):
            return self.send(vec([] if quiet else [({"namespace":"addons","stage":"external-secrets","promotion":"external-secrets.01m52gg2saeqfpr35wpkj06wn9.294f34b","phase":"Errored"},1)]))
        if q.startswith("(label_replace(sum by"):
            return self.send(vec([] if quiet else [({"namespace":"mcp-system","pod":"grafana-mcp-7d9f8-abcde","kind":"restarts"},37),({"namespace":"mcp-system","pod":"grafana-mcp-7d9f8-abcde","kind":"age"},7300),({"namespace":"authentik","pod":"authentik-worker-5c6-xyz12","kind":"age"},1300),({"namespace":"authentik","pod":"authentik-worker-5c6-xyz12","kind":"restarts"},0),({"namespace":"media","pod":"plex-sync-0","kind":"age"},200000)]))
        if q.startswith("(max by (namespace, pod, reason)"):
            return self.send(vec([] if quiet else [({"namespace":"mcp-system","pod":"grafana-mcp-7d9f8-abcde","reason":"CrashLoopBackOff"},1),({"namespace":"authentik","pod":"authentik-worker-5c6-xyz12","reason":"ContainerCreating"},1),({"namespace":"media","pod":"plex-sync-0","reason":"Unschedulable","uid":"u"},1)]))
        if "etcd-snapshot" in q:
            if q.startswith("kube_cronjob_info"): return self.send(vec([]))
            return self.send(vec([({"namespace":"talos-backup","cronjob":"talos-etcd-snapshot"},50000),({"namespace":"vcluster-media","cronjob":"vcluster-media-etcd-snapshot"},48000 if quiet else 140000)]))
        if "certmanager_certificate_ready_status" in q: return self.send(vec([] if quiet else [({"namespace":"umami","name":"stats-tls","condition":"True"},0)]))
        if q.startswith("count(certmanager"): return self.send(vec([({},14)]))
        if q.startswith("bottomk(3, (certmanager"): return self.send(vec([({"namespace":"nginx-gateway","name":"wildcard-cluster-integratn-tech"},12.4 if not quiet else 52),({"namespace":"vcluster-media","name":"wildcard-vcluster-media"},58.1),({"namespace":"authentik","name":"auth-tls"},61.0)]))
        if q.startswith("sum by (severity) (trivy"): return self.send(vec([({"severity":"Critical"},312),({"severity":"High"},1840)]))
        if q.startswith("topk(4, sum by (namespace) (trivy"): return self.send(vec([({"namespace":"authentik"},88),({"namespace":"monitoring"},61),({"namespace":"ai"},40),({"namespace":"kratix-platform-system"},12)]))
        if "kyverno_policy_results_total" in q: return self.send(vec([] if quiet else [({"policy_name":"require-requests-limits"},14),({"policy_name":"disallow-latest-tag"},2)]))
        fresh = MODE=="fresh"
        H3=["holocron","pitdroid"]
        if 'max by (host) (up{job="host-node"})' in q:
            return self.send(vec([({"host":h},1) for h in H3]+[({"host":"tower"},-1)] if not fresh else [({"host":h},-1) for h in H3+["tower"]]))
        if 'job="host-node"' in q and fresh: return self.send(vec([]))
        if "node_cpu_seconds_total{job" in q: return self.send(vec([({"host":"holocron"},34.2),({"host":"pitdroid"},71.0)]))
        if "node_memory_MemAvailable_bytes{job" in q: return self.send(vec([({"host":"holocron"},62.0),({"host":"pitdroid"},48.0)]))
        if "node_boot_time_seconds" in q: return self.send(vec([({"host":"holocron"},86400*12.3),({"host":"pitdroid"},86400*3.1)]))
        if q.startswith("max by (host, mountpoint) (100"): return self.send(vec([({"host":"holocron","mountpoint":"/System/Volumes/Data"},71.0),({"host":"holocron","mountpoint":"/nix"},71.0),({"host":"pitdroid","mountpoint":"/"},93.5)]))
        if q.startswith("max by (host, mountpoint) (node_filesystem_avail"): return self.send(vec([({"host":"holocron","mountpoint":"/System/Volumes/Data"},1.1e12),({"host":"holocron","mountpoint":"/nix"},1.1e12),({"host":"pitdroid","mountpoint":"/"},6.2e9)]))
        if "github_runner" in q:
            if fresh: return self.send(vec([]))
            R={"holocron":["runwright","specmarshal","specmarshal-2","specmarshal-3","specmarshal-4"],"pitdroid":["runwright","specmarshal","specmarshal-2","specmarshal-3","specmarshal-7","specmarshal-8"]}
            if q=="github_runner_up": return self.send(vec([({"host":h,"runner":r},0 if (h,r)==("pitdroid","specmarshal-7") else 1) for h in R for r in R[h]]))
            if q=="github_runner_busy": return self.send(vec([({"host":h,"runner":r},1 if r in ("specmarshal","specmarshal-2") and h=="holocron" else 0) for h in R for r in R[h]]))
            if q.startswith("sum by (host)"): return self.send(vec([({"host":"holocron"},5),({"host":"pitdroid"},5)]))
            if q.startswith("count by (host)"): return self.send(vec([({"host":"holocron"},5),({"host":"pitdroid"},6)]))
        if "pve_" in q:
            if fresh: return self.send(vec([]) if not "or vector(0)" in q else vec([({},0)]))
            if q.startswith("min(pve_up"): return self.send(vec([({},1)]))
            if q.startswith('pve_up{id=~"node'): return self.send(vec([({"id":"node/pve"},1)]))
            if "pve_cpu_usage_ratio" in q: return self.send(vec([({"id":"node/pve"},22.0)]))
            if "pve_memory_usage_bytes" in q: return self.send(vec([({"id":"node/pve"},83.0)]))
            if q.startswith('pve_up{id=~"(qemu|lxc)'): return self.send(vec([({"id":"qemu/100","name":"pitdroid"},1),({"id":"qemu/101","name":"homeassistant"},1),({"id":"lxc/200","name":"old-test"},0),({"id":"qemu/102","name":"onetwo"},0)]))
            if q.startswith("pve_onboot_status"): return self.send(vec([({"id":"qemu/100"},1),({"id":"qemu/101"},1),({"id":"qemu/102"},1)]))
            if "pve_disk_usage_bytes" in q: return self.send(vec([({"id":"storage/pve/local"},41.0),({"id":"storage/pve/local-lvm"},91.0)]))
            if q.startswith("count(pve_up{id=~\"(qemu|lxc)/.+\"} == 1)") or "== 1) or vector" in q: return self.send(vec([({},2)]))
            if "and on (id)" in q: return self.send(vec([({},1)]))
            if q.startswith("count(pve_up"): return self.send(vec([({},4)]))
        if "unpoller" in q:
            if fresh: return self.send(vec([]) if not "or vector(0)" in q else vec([({},0)]))
            if 'up{job="unpoller"}' in q: return self.send(vec([({},1)]))
            if q.startswith("group by (subsystem, status)"): return self.send(vec([({"subsystem":s,"status":"warning" if s=="wlan" else "ok"},1) for s in ["www","wan","lan","wlan","vpn"]]))
            if "unpoller_site_disconnected" in q: return self.send(vec([({},1)]))
            if q.startswith("sum(max by (subsystem) (unpoller_site_users"): return self.send(vec([({},64)]))
            if "unpoller_site_users" in q: return self.send(vec([({"subsystem":"lan"},18),({"subsystem":"wlan"},46)]))
            if "unpoller_device_info" in q: return self.send(vec([({},9)]))
            if "upgradeable" in q: return self.send(vec([({},2)]))
            if "latency" in q: return self.send(vec([({},14.0)]))
            if "drops" in q: return self.send(vec([({},1)]))
        if "node_cpu_seconds_total" in q or "node_memory" in q or "hwmon" in q:
            return self.send(vec([({"nodename":f"node{i}"},40+i*10) for i in range(1,4)]))
        if "kube_pod_info" in q: return self.send(vec([({"node":f"node{i}"},60) for i in range(1,4)]))
        return self.send(vec([({},12)]))
HTTPServer(("127.0.0.1",int(sys.argv[2]) if len(sys.argv)>2 else 9999),H).serve_forever()
