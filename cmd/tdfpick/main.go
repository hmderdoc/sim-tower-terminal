// tdfpick: survey TheDraw fonts for a title that fits. Dev tool only.
//
//	tdfpick <fontdir> <text> [maxcols] [name...]   (names: render those fonts as UTF-8 text)
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"simtower/internal/tdf"
)

var cp437 = []rune("\u0000☺☻♥♦♣♠•◘○◙♂♀♪♫☼►◄↕‼¶§▬↨↑↓→←∟↔▲▼ !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~⌂ÇüéâäàåçêëèïîìÄÅÉæÆôöòûùÿÖÜ¢£¥₧ƒáíóúñÑªº¿⌐¬½¼¡«»░▒▓│┤╡╢╖╕╣║╗╝╜╛┐└┴┬├─┼╞╟╚╔╩╦╠═╬╧╨╤╥╙╘╒╓╫╪┘┌█▄▌▐▀αßΓπΣσµτΦΘΩδ∞φε∩≡±≥≤⌠⌡÷≈°∙·√ⁿ²■ ")

func main() {
	dir, text := os.Args[1], os.Args[2]
	maxCols := 76
	if len(os.Args) > 3 {
		maxCols, _ = strconv.Atoi(os.Args[3])
	}
	want := map[string]bool{}
	for _, n := range os.Args[4:] {
		want[n] = true
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.tdf"))
	type hit struct {
		name string
		w, h int
	}
	var hits []hit
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for i, font := range tdf.ParseAll(data) {
			w, h := font.Measure(text)
			name := strings.TrimSuffix(filepath.Base(f), ".tdf")
			if i > 0 {
				name = fmt.Sprintf("%s#%d", name, i)
			}
			if w == 0 || w > maxCols {
				continue
			}
			hits = append(hits, hit{name, w, h})
			if want[name] {
				lines, _, _ := font.Render(text, tdf.RenderOpts{})
				fmt.Printf("== %s (%dx%d)\n", name, w, h)
				for _, ln := range lines {
					var sb strings.Builder
					esc := false
					for i := 0; i < len(ln); i++ {
						c := ln[i]
						if c == 0x1b {
							esc = true
							continue
						}
						if esc {
							if c >= 0x40 && c <= 0x7e && c != '[' {
								esc = false
							}
							continue
						}
						sb.WriteRune(cp437[c])
					}
					fmt.Println(sb.String())
				}
			}
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].h != hits[j].h && hits[i].h < hits[j].h || hits[i].h == hits[j].h && hits[i].w < hits[j].w
	})
	if len(want) == 0 {
		for _, h := range hits {
			fmt.Printf("%-14s %3dx%-2d\n", h.name, h.w, h.h)
		}
		fmt.Println(len(hits), "fonts fit")
	}
}
