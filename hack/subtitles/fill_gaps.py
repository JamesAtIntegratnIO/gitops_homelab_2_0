# fill_gaps.py <aligned.srt> <chinese.ass> <whisper.srt> <clip start s> <out.srt>
# Inserts local Whisper English cues where the BD has Chinese (CN style) lines but no English cue.
# Rules: keep a Whisper segment only if it overlaps an uncovered CN line and is <50% covered by existing
# English; snap start/end to that CN line boundary when within 1.5 s; never overlap existing cues; drop <0.3 s.
import re,sys
ts=lambda h,m,s,ms:int(h)*3600+int(m)*60+int(s)+int(ms)/1000
P=r'(\d+)\n(\d+):(\d+):(\d+),(\d+) --> (\d+):(\d+):(\d+),(\d+)\n(.*?)(?=\n\n\d+\n\d+:\d+:\d+,|\Z)'
rd=lambda p:[(ts(*b[1:5]),ts(*b[5:9]),b[9].strip()) for b in re.findall(P,open(p,encoding='utf-8',errors='replace').read().replace('\r',''),re.S)]
fmt=lambda s:"%02d:%02d:%02d,%03d"%(s//3600,s%3600//60,int(s%60),round((s%1)*1000)%1000)
cur=rd(sys.argv[1]); wh=rd(sys.argv[3]); off=float(sys.argv[4])
cn=sorted((int(a)*3600+int(b)*60+float(c),int(d)*3600+int(e)*60+float(g)) for a,b,c,d,e,g in re.findall(r'^Dialogue: \d+,(\d+):(\d+):([\d.]+),(\d+):(\d+):([\d.]+),CN,',open(sys.argv[2],encoding='utf-8',errors='replace').read(),re.M))
lo=off;hi=off+max(b for a,b,_ in wh)+1
ovl=lambda a,b,x,y:max(0,min(b,y)-max(a,x))
free=[(a,b) for a,b in cn if lo<=a<=hi and not any(ovl(a,b,x,y)>0 for x,y,_ in cur)]
new=[];drop={'covered':0,'no_cn':0,'noise':0}
for a,b,t in wh:
    a+=off;b+=off;t=re.sub(r'\s+',' ',t).strip()
    if not t or re.fullmatch(r'[\W_]*',t) or re.search(r'(?i)thank you for watching|subscribe',t): drop['noise']+=1;continue
    if sum(ovl(a,b,x,y) for x,y,_ in cur)>0.5*(b-a): drop['covered']+=1;continue
    hits=[c for c in free if ovl(a-0.5,b+0.5,*c)>0]
    if not hits: drop['no_cn']+=1;continue
    s=min(hits,key=lambda c:abs(c[0]-a)); e=min(hits,key=lambda c:abs(c[1]-b))
    na=s[0] if abs(s[0]-a)<=1.5 else a; nb=e[1] if abs(e[1]-b)<=1.5 else b
    new.append([na,max(nb,na+1.0),t])
allc=sorted([list(c) for c in cur]+new)
for x,y in zip(allc,allc[1:]):
    if x[1]>y[0]: x[1]=y[0]
isnew={id(n) for n in new}
out=[c for c in allc if c[1]-c[0]>=0.3 or id(c) not in isnew]
added=[c for c in out if id(c) in isnew]
cov=sum(1 for c in free if any(ovl(*c,n[0],n[1])>0 for n in added))
print(f"added {len(added)} cues, dropped {drop}, uncovered CN lines in window {len(free)}, now covered {cov}, range {fmt(added[0][0]) if added else '-'}-{fmt(added[-1][1]) if added else '-'}, cues {len(cur)}->{len(out)}")
open(sys.argv[5],'w',encoding='utf-8').write("\n".join(f"{k+1}\n{fmt(a)} --> {fmt(b)}\n{t}\n" for k,(a,b,t) in enumerate(out)))
