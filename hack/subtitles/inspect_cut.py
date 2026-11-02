# inspect_cut.py EP off1 off2 lo hi: print English cues between lo..hi s with nearest CN line under two offsets,
# to place a cut by reading both tracks.
import re,sys,bisect
ts=lambda h,m,s,ms:int(h)*3600+int(m)*60+int(s)+int(ms)/1000
P=r'(\d+)\n(\d+):(\d+):(\d+),(\d+) --> (\d+):(\d+):(\d+),(\d+)\n(.*?)(?=\n\n\d+\n\d+:\d+:\d+,|\Z)'
f=lambda s:"%d:%05.2f"%(s//60,s%60)
ep,o1,o2,lo,hi=sys.argv[1],float(sys.argv[2]),float(sys.argv[3]),float(sys.argv[4]),float(sys.argv[5])
en=sorted((ts(*b[1:5]),b[9].strip().replace('\n',' ')) for b in re.findall(P,open(f'E{ep}.srt',encoding='utf-8',errors='replace').read().replace('\r',''),re.S))
cn=sorted((int(a)*3600+int(b)*60+float(c),re.sub(r'{[^}]*}','',t)) for a,b,c,t in re.findall(r'^Dialogue: \d+,(\d+):(\d+):([\d.]+),[^,]+,CN,[^,]*,[^,]*,[^,]*,[^,]*,[^,]*,(.*)$',open(f'E{ep}.ass.out',encoding='utf-8',errors='replace').read(),re.M))
cs=[c[0] for c in cn]
def near(x):
    i=bisect.bisect(cs,x);j=min(range(max(i-1,0),min(i+1,len(cs))),key=lambda k:abs(cs[k]-x));return cs[j]-x,cn[j][1]
for k,(a,t) in enumerate(en):
    if lo<=a<=hi:
        d1,c1=near(a+o1);d2,c2=near(a+o2)
        print(k,f(a),"%-40s"%t[:40],"| o1 %+.2f %s | o2 %+.2f %s"%(d1,c1[:14],d2,c2[:14]))
