# fixed_offset.py EP...: best single offset (0.00-2.00 s, 0.01 s steps) by overlap with the CN track;
# drops {...}-only typesetting cues; writes E<EP>.final.srt and prints 4-min region medians.
import re,sys,bisect,statistics as st
ts=lambda h,m,s,ms:int(h)*3600+int(m)*60+int(s)+int(ms)/1000
P=r'(\d+)\n(\d+):(\d+):(\d+),(\d+) --> (\d+):(\d+):(\d+),(\d+)\n(.*?)(?=\n\n\d+\n\d+:\d+:\d+,|\Z)'
fmt=lambda s:"%02d:%02d:%02d,%03d"%(s//3600,s%3600//60,int(s%60),round((s%1)*1000)%1000)
def load(ep):
    en=sorted((ts(*b[1:5]),ts(*b[5:9]),b[9].strip()) for b in re.findall(P,open(f'E{ep}.srt',encoding='utf-8',errors='replace').read().replace('\r',''),re.S))
    cn=sorted((int(a)*3600+int(b)*60+float(c),int(d)*3600+int(e)*60+float(g)) for a,b,c,d,e,g in re.findall(r'^Dialogue: \d+,(\d+):(\d+):([\d.]+),(\d+):(\d+):([\d.]+),CN,',open(f'E{ep}.ass.out',encoding='utf-8',errors='replace').read(),re.M))
    return en,cn
def regions(en,cn,off):
    cs=[c[0] for c in cn];d={}
    for a,b,t in en:
        if re.fullmatch(r'\s*\{[^}]*\}\s*',t):continue
        x=a+off;i=bisect.bisect(cs,x);c=min(cs[max(i-1,0):i+1],key=lambda y:abs(y-x));d.setdefault(int(x//240)*4,[]).append(c-x)
    return {k:round(st.median(v),2) for k,v in sorted(d.items()) if len(v)>=3}
for ep in sys.argv[1:]:
    en,cn=load(ep)
    G=0.01;N=int((cn[-1][1]+30)/G);cov=bytearray(N)
    for a,b in cn:
        cov[int(a/G):min(int(b/G),N)]=b'\x01'*(min(int(b/G),N)-int(a/G))
    def score(o):
        s=0
        for a,b,t in en:
            i=max(0,int((a+o)/G));j=min(N,int((b+o)/G));s+=sum(cov[i:j])
        return s
    o=max((x/100 for x in range(0,200)),key=score)
    keep=[(a,b,t) for a,b,t in en if not re.fullmatch(r'\s*\{[^}]*\}\s*',t)]
    r=regions(keep,cn,o)
    print(f"E{ep} offset {o:+.2f} cues {len(en)}->{len(keep)} regions {r} max|{max(abs(v) for v in r.values()):.2f}|")
    open(f'E{ep}.final.srt','w',encoding='utf-8').write("\n".join(f"{k+1}\n{fmt(a+o)} --> {fmt(b+o)}\n{t}\n" for k,(a,b,t) in enumerate(keep)))
