# piecewise_offset.py EP idx:offset ...: apply hand-chosen cut points (English cue index) with offsets,
# each refined +-0.3 s by overlap; collapses repeated sign cues; writes E<EP>.final.srt; prints regions.
# Acceptance: every 4-min region median within 0.3 s of the CN track.
# usage: piece.py EP idx:offset idx:offset ...  (segment starts at en cue index; offsets refined +-0.3 s)
import re,sys,bisect,statistics as st
ts=lambda h,m,s,ms:int(h)*3600+int(m)*60+int(s)+int(ms)/1000
P=r'(\d+)\n(\d+):(\d+):(\d+),(\d+) --> (\d+):(\d+):(\d+),(\d+)\n(.*?)(?=\n\n\d+\n\d+:\d+:\d+,|\Z)'
fmt=lambda s:"%02d:%02d:%02d,%03d"%(s//3600,s%3600//60,int(s%60),round((s%1)*1000)%1000)
f=lambda s:"%d:%05.2f"%(s//60,s%60)
ep=sys.argv[1]; segs=[(int(a),float(b)) for a,b in (x.split(':') for x in sys.argv[2:])]
en=sorted((ts(*b[1:5]),ts(*b[5:9]),b[9].strip()) for b in re.findall(P,open(f'E{ep}.srt',encoding='utf-8',errors='replace').read().replace('\r',''),re.S))
cn=sorted((int(a)*3600+int(b)*60+float(c),int(d)*3600+int(e)*60+float(g)) for a,b,c,d,e,g in re.findall(r'^Dialogue: \d+,(\d+):(\d+):([\d.]+),(\d+):(\d+):([\d.]+),CN,',open(f'E{ep}.ass.out',encoding='utf-8',errors='replace').read(),re.M))
G=0.01;N=int((max(cn[-1][1],en[-1][1])+300)/G);cov=bytearray(N)
for a,b in cn: cov[int(a/G):min(int(b/G),N)]=b'\x01'*(min(int(b/G),N)-int(a/G))
sign=lambda t:re.fullmatch(r'\s*\{[^}]*\}\s*',t) is not None
offs=[]
for k,(s,o) in enumerate(segs):
    e=segs[k+1][0] if k+1<len(segs) else len(en)
    part=[x for x in en[s:e] if not sign(x[2])]
    sc=lambda d:sum(sum(cov[max(0,int((a+d)/G)):min(N,int((b+d)/G))]) for a,b,_ in part)
    best=max((round(o+i/100,2) for i in range(-30,31)),key=sc); offs.append((s,e,best))
    print(f" seg cues {s}-{e-1} en {f(en[s][0])} offset {best:+.2f}")
out=[]
for s,e,o in offs:
    out+=[(a+o,b+o,t) for a,b,t in en[s:e] if not sign(t)]
out.sort()
# merge identical touching cues first
t2=[]
for a,b,t in out:
    if t2 and t2[-1][2]==t and a<=t2[-1][1]+0.25: t2[-1]=(t2[-1][0],max(b,t2[-1][1]),t)
    else: t2.append((a,b,t))
out=t2
# collapse frame-by-frame typeset sign cues: a text repeated within 3 s becomes one cue spanning all repeats
m=[];last={}
for a,b,t in out:
    k=last.get(t)
    if k is not None and a<=m[k][1]+3.0 and b-a<0.5:
        m[k]=(m[k][0],max(b,m[k][1]),t)
    else:
        last[t]=len(m); m.append((a,b,t))
out=sorted(m)
for x,y in zip(out,out[1:]):
    pass
cs=[c[0] for c in cn];d={}
for a,b,t in out:
    i=bisect.bisect(cs,a);c=min(cs[max(i-1,0):i+1],key=lambda y:abs(y-a));d.setdefault(int(a//240)*4,[]).append(c-a)
r={k:round(st.median(v),2) for k,v in sorted(d.items()) if len(v)>=3}
print(f"E{ep} cues {len(en)}->{len(out)} regions {r} max|{max(abs(v) for v in r.values()):.2f}|")
open(f'E{ep}.final.srt','w',encoding='utf-8').write("\n".join(f"{k+1}\n{fmt(max(0,a))} --> {fmt(max(0,b))}\n{t}\n" for k,(a,b,t) in enumerate(out)))
