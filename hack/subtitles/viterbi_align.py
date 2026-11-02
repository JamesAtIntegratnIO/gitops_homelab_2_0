# viterbi_align.py EP...: first-pass piecewise alignment of E<EP>.srt to the BD's Chinese track E<EP>.ass.out
# (CN style). Constant offset per segment, switch penalty 3 s, offsets -8..+200 s in 0.1 s steps,
# offsets may only increase (BD adds material). Prints 4-min region medians, segments and gaps.
import re,sys,bisect,statistics as st,html
G=0.05;P=3.0
ts=lambda h,m,s,ms:int(h)*3600+int(m)*60+int(s)+int(ms)/1000
f=lambda s:"%d:%04.1f"%(s//60,s%60)
def run(ep):
    raw=open(f'E{ep}.srt',encoding='utf-8',errors='replace').read().replace('\r','')
    bl=re.findall(r'(\d+)\n(\d+):(\d+):(\d+),(\d+) --> (\d+):(\d+):(\d+),(\d+)\n(.*?)(?=\n\n\d+\n\d+:\d+:\d+,|\Z)',raw,re.S)
    en=sorted((ts(*b[1:5]),ts(*b[5:9]),b[9].strip()) for b in bl)
    A=open(f'E{ep}.ass.out',encoding='utf-8',errors='replace').read()
    cn=[]
    for a,b,c,d,e,g,tx in re.findall(r'^Dialogue: \d+,(\d+):(\d+):([\d.]+),(\d+):(\d+):([\d.]+),CN,[^,]*,[^,]*,[^,]*,[^,]*,[^,]*,(.*)$',A,re.M):
        cn.append((int(a)*3600+int(b)*60+float(c),int(d)*3600+int(e)*60+float(g),re.sub(r'{[^}]*}','',tx)))
    cn.sort(); cs=[c[0] for c in cn]
    T=int(max(cn[-1][1],en[-1][1])+260);N=int(T/G)+2;cov=[0]*N
    for a,b,_ in cn:
        for i in range(int(a/G),min(int(b/G)+1,N-1)):cov[i]=1
    C=[0]*(N+1)
    for i in range(N):C[i+1]=C[i]+cov[i]
    offs=[round(-8+i*0.1,2) for i in range(int(208/0.1)+1)]
    def ov(a,b,o):
        i=max(0,int((a+o)/G));j=min(N-1,int((b+o)/G));return (C[j]-C[i])*G if j>i else 0
    best=[0.0 if abs(o)<8.01 else -1e9 for o in offs];back=[]
    for a,b,_ in en:
        bk=[];nb=[];m=-1e18;mi=0
        for k,o in enumerate(offs):
            sw=m-P
            if best[k]>=sw: bk.append(k);v=best[k]
            else: bk.append(mi);v=sw
            nb.append(v+ov(a,b,o))
            if best[k]>m:m=best[k];mi=k
        best=nb;back.append(bk)
    k=best.index(max(best));path=[0]*len(en)
    for i in range(len(en)-1,-1,-1):path[i]=offs[k];k=back[i][k]
    segs=[]
    for i,o in enumerate(path):
        if not segs or segs[-1][2]!=o:segs.append([i,i,o])
        else:segs[-1][1]=i
    def reg(pts):
        d={}
        for x in pts:
            i=bisect.bisect(cs,x);c=min(cs[max(i-1,0):i+1],key=lambda y:abs(y-x));d.setdefault(int(x//240),[]).append(c-x)
        return {m*4:round(st.median(v),2) for m,v in sorted(d.items()) if len(v)>=3}
    raww=reg([e[0] for e in en]); al=reg([e[0]+o for e,o in zip(en,path)])
    print(f"== E{ep} {open(f'E{ep}.rel').read()[:48]} en={len(en)} cn={len(cn)} en_last={f(en[-1][0])} cn_last={f(cn[-1][0])}")
    print(" raw 4-min medians",raww)
    print(" segs",[(f(en[s][0]),o) for s,e,o in segs]);print(" aligned",al)
    # english gaps against CN after alignment
    ae=sorted((e[0]+o,e[1]+o) for e,o in zip(en,path)); g=[]; 
    run_s=None
    for a,b,_ in cn:
        hit=any(x<b and y>a for x,y in ae)
        if not hit:
            run_s=run_s if run_s else a; last=b
        else:
            if run_s and last-run_s>=15:g.append((f(run_s),f(last)))
            run_s=None
    if run_s and last-run_s>=15:g.append((f(run_s),f(last)))
    print(" cn-only gaps>=15s",g)
    # text at segment starts
    for s,e,o in segs[:6]:
        x=en[s][0]+o;c=min(cn,key=lambda r:abs(r[0]-x));print("  seg",f(x),en[s][2].replace("\n"," ")[:44],"|",f(c[0]),c[2][:22])
    o2=[]
    fmt=lambda s:"%02d:%02d:%02d,%03d"%(s//3600,s%3600//60,int(s%60),round((s%1)*1000)%1000)
    for i,((a,b,tx),o) in enumerate(zip(en,path)):o2.append(f"{i+1}\n{fmt(max(0,a+o))} --> {fmt(max(0,b+o))}\n{tx}\n")
    open(f'E{ep}.aligned.srt','w',encoding='utf-8').write("\n".join(o2))
for e in sys.argv[1:]:run(e)
