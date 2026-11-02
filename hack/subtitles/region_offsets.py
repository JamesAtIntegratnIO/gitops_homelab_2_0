# region_offsets.py en.srt chinese.ass: median nearest-cue delta (CN - EN) per 2-min window, with IQR.
import re,sys,statistics as st,bisect
def srt(f):
    t=open(f,encoding='utf-8',errors='replace').read()
    return [int(a)*3600+int(b)*60+int(c)+int(d)/1000 for a,b,c,d in re.findall(r'(\d+):(\d+):(\d+),(\d+) -->',t)]
def ass(f,style='CN'):
    return sorted(int(a)*3600+int(b)*60+float(c) for a,b,c in re.findall(r'^Dialogue: \d+,(\d+):(\d+):([\d.]+),[^,]+,'+style+',',open(f,encoding='utf-8',errors='replace').read(),re.M))
en=srt(sys.argv[1]); cn=ass(sys.argv[2])
print("en",len(en),"last %.0f"%en[-1],"cn",len(cn),"last %.0f"%cn[-1])
print("min | n | median cn-en | IQR")
for m in range(0,int(max(en[-1],cn[-1])//60)+1,2):
    o=[]
    for x in en:
        if m*60<=x<(m+2)*60:
            i=bisect.bisect(cn,x); c=min(cn[max(i-1,0):i+1],key=lambda y:abs(y-x)); o.append(c-x)
    if o:
        q=st.quantiles(o,n=4) if len(o)>1 else [o[0],o[0],o[0]]
        print("%02d-%02d %3d %+7.2f  [%+.2f,%+.2f]"%(m,m+2,len(o),st.median(o),q[0],q[2]))
