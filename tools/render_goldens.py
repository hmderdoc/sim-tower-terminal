#!/usr/bin/env python3
"""Render internal/ui/testdata/<name>.txt golden dumps to PNG with the IBM VGA
8x16 font (tools/CP437.F16), for the README and for reviewing the art.

    python3 tools/render_goldens.py 80x24_main 132x50_main ...   -> docs/screenshots/<name>.png
"""
import os, sys
from PIL import Image
PAL=["000000","0000AA","00AA00","00AAAA","AA0000","AA00AA","AA5500","AAAAAA","555555","5555FF","55FF55","55FFFF","FF5555","FF55FF","FFFF55","FFFFFF"]
RGB=[tuple(int(h[i:i+2],16) for i in (0,2,4)) for h in PAL]
HERE=os.path.dirname(os.path.abspath(__file__))
ROOT=os.path.dirname(HERE)
font=open(os.path.join(HERE,'CP437.F16'),'rb').read()
LOW='\u0000\u263a\u263b\u2665\u2666\u2663\u2660\u2022\u25d8\u25cb\u25d9\u2642\u2640\u266a\u266b\u263c\u25ba\u25c4\u2195\u203c\u00b6\u00a7\u25ac\u21a8\u2191\u2193\u2192\u2190\u221f\u2194\u25b2\u25bc'
LOWMAP={c:i for i,c in enumerate(LOW) if i}
LOWMAP['\u2302']=127
def load(path):
    secs={}; cur=None
    for ln in open(path,encoding='utf-8'):
        ln=ln.rstrip('\n')
        if ln.startswith('# '): cur=ln[2:].split()[0]; secs[cur]=[]; continue
        if cur: secs[cur].append(ln)
    return secs['chars'], secs['fg'], secs['bg']
def render(path,out,scale=1):
    ch,fg,bg=load(path)
    rows=len(ch); cols=max(len(r) for r in ch)
    im=Image.new('RGB',(cols*8,rows*16)); px=im.load()
    for y in range(rows):
        for x in range(cols):
            c=ch[y][x] if x<len(ch[y]) else ' '
            f=int(fg[y][x],16) if x<len(fg[y]) else 7
            k=int(bg[y][x],16) if x<len(bg[y]) else 0
            if c in LOWMAP: b=LOWMAP[c]
            else:
                try: b=c.encode('cp437')[0]
                except Exception: b=0x3F
            for gy in range(16):
                bits=font[b*16+gy]
                for gx in range(8):
                    px[x*8+gx,y*16+gy]=RGB[f] if bits&(0x80>>gx) else RGB[k]
    if scale>1: im=im.resize((im.width*scale,im.height*scale),Image.NEAREST)
    im.save(out)
for name in sys.argv[1:]:
    render(os.path.join(ROOT,'internal','ui','testdata',name+'.txt'), os.path.join(ROOT,'docs','screenshots',name+'.png'), 1)
