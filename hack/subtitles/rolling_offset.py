# rolling_offset.py EP...: best offset per 60 s window (+-0.25 s match tolerance) to spot cuts and inserts.
import re,sys,bisect,statistics as st
ts=lambda h,m,s,ms:int(h)*3600+int(m)*60+int(s)+int(ms)/1000
f=lambda s:"%d:%04.1f"%(s//60,s%60)
def load(ep):
    raw=open(f'E{ep}.srt',encoding='utf-8',errors='replace').read().replace('\r','')
    bl=re.findall(r'(\d+)\n(\d+):(\d+):(\d+),(\d+) --> (\d+):(\d+):(\d+),(\d+)\n(.*?)(?=\n\n\d+\n\d+:\d+:\d+,|\Z)',raw,re.S)
    en=sorted((ts(*b[1:5]),ts(*b[5:9]),b[9].strip().replace('\n',' ')) for b in bl)
    A=open(f'E{ep}.ass.out',encoding='utf-8',errors='replace').read()
    cn=sorted((int(a)*3600+int(b)*60+float(c),re.sub(r'{[^}]*}','',t)) for a,b,c,t in re.findall(r'^Dialogue: \d+,(\d+):(\d+):([\d.]+),[^,]+,CN,[^,]*,[^,]*,[^,]*,[^,]*,[^,]*,(.*)$',A,re.M))
    return en,cn
def rolling(ep,win=60):
    en,cn=load(ep);cs=[c[0] for c in cn]
    # histogram-mode offset per window: best delta within +-200 s by matching count at 0.2s tolerance
    out=[]
    e=[x[0] for x in en if not re.search(r'[{]',x[2])]
    t=0
    while t<e[-1]:
        pts=[x for x in e if t<=x<t+win]
        if len(pts)>=5:
            best=None
            for d10 in range(-50,2100):
                d=d10/10;n=0
                for x in pts:
                    i=bisect.bisect(cs,x+d)
                    if min((abs(c-(x+d)) for c in cs[max(i-1,0):i+1]),default=9)<0.25:n+=1
                if not best or n>best[0]:best=(n,d)
            out.append((f(t),best[1],"%d/%d"%(best[0],len(pts))))
        t+=win
    return out
for ep in sys.argv[1:]:
    print("==",ep);print(rolling(ep))
