# os_download_candidates.py: run INSIDE the Bazarr pod (kubectl exec -i ... python3 - < this).
# For UBW (parent_imdb_id 3621796) S01E01-12, picks Commie > UTW/BD > others on OpenSubtitles.com and
# saves /tmp/ubw/EXX.srt + .rel. Uses Bazarr's own provider API key and stored login; prints none of them.
import re,json,urllib.request,urllib.error,subprocess,os,time
src=open('/app/bazarr/bin/bazarr/app/get_providers.py').read()
K=re.search(r"opensubtitlescom['\"]?\s*:\s*\{.*?api_key['\"]?\s*:\s*['\"]([^'\"]+)",src,re.S)
if not K:
    K=re.search(r"api_key\s*=\s*['\"]([^'\"]+)",open('/app/bazarr/bin/custom_libs/subliminal_patch/providers/opensubtitlescom.py').read())
K=K.group(1)
sec=re.search(r"^opensubtitlescom:\n((?:[ ].*\n)+)",open('/config/config/config.yaml').read(),re.M).group(1)
v=lambda n: re.search(r"^ *%s: *(.*)$"%n,sec,re.M).group(1).strip().strip("'\"")
H={"Api-Key":K,"User-Agent":"Bazarr v1.6.2","Accept":"application/json","Content-Type":"application/json"}
def call(url,body=None,hdr={}):
    r=urllib.request.Request(url,data=json.dumps(body).encode() if body is not None else None,headers={**H,**hdr})
    with urllib.request.urlopen(r,timeout=30) as f: return json.load(f)
base="https://api.opensubtitles.com/api/v1"
l=call(base+"/login",{"username":v("username"),"password":v("password")}); b=l.get("base_url") or "api.opensubtitles.com"
os.makedirs("/tmp/ubw",exist_ok=True)
for ep in range(1,13):
    time.sleep(1)
    d=call(base+"/subtitles?episode_number=%d&languages=en&parent_imdb_id=3621796&season_number=1"%ep)["data"]
    rank=lambda x:(0 if 'commie' in str(x["attributes"].get("release")).lower() else 1 if re.search('utw|bd|blu',str(x["attributes"].get("release")),re.I) else 2,-x["attributes"].get("download_count",0))
    d=sorted(d,key=rank)
    print("E%02d"%ep,"candidates",[str(x["attributes"].get("release"))[:42] for x in d])
    if not d: continue
    c=d[0]["attributes"]; fid=c["files"][0]["file_id"]
    r=call("https://"+b+"/api/v1/download",{"file_id":fid,"sub_format":"srt"},{"Authorization":"Bearer "+l["token"]})
    p="/tmp/ubw/E%02d.srt"%ep
    subprocess.run(["curl","-sS","-m","30","-A","Bazarr v1.6.2","-L","-o",p,r["link"]],check=True)
    print("  got",c.get("release"),os.path.getsize(p),"remaining",r.get("remaining"))
    open(p+".rel","w").write(str(c.get("release")))
