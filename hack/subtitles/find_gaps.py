# find_gaps.py aligned.srt chinese.ass: runs of >=2 CN lines spanning >=8 s with no English (BD-only scenes).
# list runs of Chinese (CN style) lines with no overlapping English cue: candidate BD-only scenes
import re,sys
ts=lambda h,m,s,ms:int(h)*3600+int(m)*60+int(s)+int(ms)/1000
f=lambda s:"%d:%05.2f"%(s//60,s%60)
srt,ass=sys.argv[1],sys.argv[2]
en=[(ts(*x[0:4]),ts(*x[4:8])) for x in re.findall(r'(\d+):(\d+):(\d+),(\d+) --> (\d+):(\d+):(\d+),(\d+)',open(srt).read())]
cn=sorted((int(a)*3600+int(b)*60+float(c),int(d)*3600+int(e)*60+float(g)) for a,b,c,d,e,g in re.findall(r'^Dialogue: \d+,(\d+):(\d+):([\d.]+),(\d+):(\d+):([\d.]+),CN,',open(ass,encoding='utf-8',errors='replace').read(),re.M))
run=[];gaps=[]
for a,b in cn+[(1e9,1e9)]:
    if a<1e9 and not any(x<b and y>a for x,y in en): run.append((a,b)); continue
    if len(run)>=2 and run[-1][1]-run[0][0]>=8: gaps.append((round(run[0][0],2),round(run[-1][1],2),len(run)))
    run=[]
for a,b,n in gaps: print(f"gap {f(a)}-{f(b)} ({a}-{b}) cn_lines={n}")
