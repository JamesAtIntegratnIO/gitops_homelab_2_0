# check_song_lines.py EP STYLE off lo hi: compare English OP/ED lyric cues against the BD's OPC/EDC lines.
import re,sys,bisect,statistics as st
ts=lambda h,m,s,ms:int(h)*3600+int(m)*60+int(s)+int(ms)/1000
P=r'(\d+)\n(\d+):(\d+):(\d+),(\d+) --> (\d+):(\d+):(\d+),(\d+)\n(.*?)(?=\n\n\d+\n\d+:\d+:\d+,|\Z)'
f=lambda s:"%d:%05.2f"%(s//60,s%60)
ep=sys.argv[1]; style=sys.argv[2]; off=float(sys.argv[3]); lo,hi=float(sys.argv[4]),float(sys.argv[5])
en=sorted((ts(*b[1:5]),b[9].strip().replace('\n',' ')) for b in re.findall(P,open(f'E{ep}.srt',encoding='utf-8',errors='replace').read().replace('\r',''),re.S))
ly=sorted(set(int(a)*3600+int(b)*60+float(c) for a,b,c in re.findall(r'^Dialogue: \d+,(\d+):(\d+):([\d.]+),[^,]+,'+style+',',open(f'E{ep}.ass.out',encoding='utf-8',errors='replace').read(),re.M)))
print(f"E{ep} {style} lines {len(ly)} span {f(ly[0])}-{f(ly[-1])}")
seen=[];
for a,t in en:
    if lo<=a<=hi and (not seen or seen[-1][1]!=t): seen.append((a,t))
for a,t in seen[:14]:
    x=a+off;c=min(ly,key=lambda y:abs(y-x)); print(" ",f(a),"->",f(x),"%+.2f"%(c-x),t[:40])
d=[min(ly,key=lambda y:abs(y-(a+off)))-(a+off) for a,t in seen]; print(" median %+.2f"%st.median(d) if d else "none")
