package ui

import (
	"strings"

	"simtower/internal/screen"
)

// Tiles are ported 1:1 from TILES in docs/graphics_plan.html. A tile is rows
// of glyphs plus parallel paint rows; each paint byte maps through the shared
// palette C to [fg,bg]. '.' in paint is transparent, as is a blank glyph with
// a blank paint byte. Art rows are w-1 wide; Blit adds the dark gray wall
// column unless the tile is wall-less (lobby, stairs, escalator) or a shaft
// car. Height is 3 rows per floor.
//
// Static tiles are plain data; animated / state-dependent tiles are small
// draw functions of (state, t). Everything is rendered once at init into a
// per-(kind, state, frame) cell cache, so Blit is a copy.

// TState is a tile state (the mock's state strings).
type TState uint8

const (
	TOccupied TState = iota
	TVacant
	TNight
	TSold
	TForSale
	TAsleep
	TDirty
	TInfested
	TVIP
	TOpen
	TClosed
	TRush
	TShowing
	TIdle
	TParty
	TDay
	TCar
	TEmpty
	TTrain
	TUsed
	TRun
	TMoving
	TWedding
	TBurning // procedural: flames over the footprint
	TBurned  // procedural: charred footprint
	numTStates
)

var tstateNames = [numTStates]string{"occupied", "vacant", "night", "sold", "forsale", "asleep",
	"dirty", "infested", "vip", "open", "closed", "rush", "showing", "idle", "party", "day", "car",
	"empty", "train", "used", "run", "moving", "wedding", "burning", "burned"}

func (s TState) String() string { return tstateNames[s] }

// pal is the mock's shared paint map C: key -> fg, bg.
var pal = func() [128][2]uint8 {
	var p [128][2]uint8
	for i := range p {
		p[i] = [2]uint8{7, 0} // C[pk] || [7,0]
	}
	set := func(k byte, fg, bg uint8) { p[k] = [2]uint8{fg, bg} }
	set('w', 11, 1) // window, day
	set('W', 14, 0) // window lit, night
	set('x', 8, 0)  // window dark/unlit
	set('p', 15, 0) // person in room
	set('P', 0, 6)  // person on lobby floor
	set('m', 7, 0)  // monitor / fixture
	set('d', 6, 0)  // desk / wood
	set('k', 8, 0)  // dim fixture
	set('b', 9, 0)  // bed
	set('B', 1, 0)  // bed base
	set('c', 15, 0) // pillow
	set('s', 13, 0) // sofa
	set('l', 10, 0) // plant
	set('y', 14, 0) // lamp / gold
	set('r', 12, 0) // red
	set('g', 10, 0) // green
	set('G', 2, 0)
	set('z', 11, 0) // zzz
	set('t', 6, 0)  // dirt / brown
	set('T', 14, 6) // text on brown sign
	set('n', 15, 5) // text on magenta sign
	set('N', 5, 0)  // magenta fixture
	set('R', 15, 4) // text on red sign
	set('o', 0, 6)  // black on amber
	set('S', 0, 7)  // black on light gray sign
	set('h', 7, 0)  // light gray
	set('H', 15, 0) // bright white
	set('e', 12, 0) // flame red
	set('E', 14, 0) // flame yellow
	set('a', 3, 0)  // dark cyan
	set('A', 11, 0) // bright cyan
	set('u', 4, 0)  // dark red
	set('v', 13, 0) // bright magenta
	set('q', 1, 0)  // dark blue
	set('Q', 9, 0)  // bright blue
	return p
}()

const wallFg = 8

type tileDef struct {
	name   string
	w, h   int // units, rows (3 per floor)
	noWall bool
	shaft  bool
	states []TState
	period int // animation period in frames (1 = static)
	draw   func(st TState, t int) (rows, paint []string)
}

// artCell is one cached cell; on=false is transparent.
type artCell struct {
	c  screen.Cell
	on bool
}

type art struct {
	w, h  int // w includes the wall column
	cells []artCell
}

const animFrames = 24 // lcm of every tile period (2,3,4,6,8)

var (
	tiles     [numKinds]*tileDef
	tileCache [numKinds][numTStates]*[animFrames]art
)

func r3(a, b, c string) []string { return []string{a, b, c} }
func rep(s string, n int) string { return strings.Repeat(s, n) }

// centerText pads s to w (mock: left = floor(pad/2)).
func centerText(s string, w int) string {
	n := len([]rune(s))
	pad := w - n
	if pad < 0 {
		pad = 0
	}
	l := pad / 2
	return rep(" ", l) + s + rep(" ", pad-l)
}

func padEnd(s string, n int, c string) string {
	k := len([]rune(s))
	if k >= n {
		return s
	}
	return s + rep(c, n-k)
}

func setRune(s string, i int, r rune) string {
	rs := []rune(s)
	if i >= 0 && i < len(rs) {
		rs[i] = r
	}
	return string(rs)
}

func init() {
	tiles[KindOffice] = &tileDef{name: "Office", w: 9, h: 3, states: []TState{TOccupied, TVacant, TNight}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			win := "w"
			if st == TNight {
				win = "W"
			} else if st == TVacant {
				win = "x"
			}
			work := st == TOccupied || st == TNight
			mid, pm := "  ·  ·  ", "  k  k  "
			if work {
				mid, pm = "☺■ ☺■ ☺■", "pm pm pm"
			}
			bot := "dd dd dd"
			if st == TVacant {
				bot = "kk kk kk"
			}
			w2 := win + win
			return r3("▄▄ ▄▄ ▄▄", mid, "▀▀ ▀▀ ▀▀"), r3(w2+" "+w2+" "+w2, pm, bot)
		}}
	tiles[KindCondo] = &tileDef{name: "Condo", w: 16, h: 3, states: []TState{TSold, TForSale, TNight}, period: 8,
		draw: func(st TState, t int) ([]string, []string) {
			win := "w"
			if st == TNight {
				win = "W"
			} else if st == TForSale {
				win = "x"
			}
			if st == TForSale {
				return r3("▄▄▄ ▄▄▄   ▄▄▄ ", " FOR SALE $150K", "               "),
					r3("xxx xxx   xxx ", " yyyyyyyyyyyyyy", "               ")
			}
			home := (t>>2)%2 == 0
			h := " "
			if home {
				h = "☺"
			}
			w3 := rep(win, 3)
			return r3("▄▄▄ ▄▄▄   ▄▄▄ ", h+" ▬▬▬ ■  ♣   ☺ ", "  ▀▀▀ ▀   ▀   ▀ "),
				r3(w3+" "+w3+"   "+w3+" ", "p sss m  l   p ", "  ddd d   d   d ")
		}}
	tiles[KindHotelSingle] = &tileDef{name: "Hotel single", w: 4, h: 3, states: []TState{TVacant, TOccupied, TAsleep, TDirty, TInfested}, period: 4,
		draw: func(st TState, t int) ([]string, []string) {
			a, pa := " ▪ ", " d "
			switch st {
			case TOccupied:
				a, pa = "☺▪ ", "pd "
			case TAsleep:
				a = "z  "
				if t%4 < 2 {
					a = " z "
				}
				pa = "zz "
			case TDirty:
				a, pa = " ~ ", " t "
			case TInfested:
				a = " ∙ "
				if t%2 == 1 {
					a = "∙ ∙"
				}
				pa = "ttt"
			}
			bed, pb := "▀▄▄", "cbb"
			if st == TDirty {
				bed, pb = "▒▒▒", "ttt"
			} else if st == TInfested {
				bed, pb = "░░░", "ttt"
			}
			return r3(a, bed, "▀▀▀"), r3(pa, pb, "BBB")
		}}
	tiles[KindHotelTwin] = &tileDef{name: "Hotel twin", w: 6, h: 3, states: []TState{TVacant, TAsleep, TDirty}, period: 4,
		draw: func(st TState, t int) ([]string, []string) {
			a, pa := " ▪  ▪", " d  d"
			if st == TAsleep {
				a = "z  z "
				if t%4 < 2 {
					a = " z  z"
				}
				pa = "zzzzz"
			} else if st == TDirty {
				a, pa = " ~  ~", "ttttt"
			}
			b, pb := "▀▄ ▀▄", "cb cb"
			if st == TDirty {
				b, pb = "▒▒ ▒▒", "ttttt"
			}
			return r3(a, b, "▀▀ ▀▀"), r3(pa, pb, "BB BB")
		}}
	tiles[KindHotelSuite] = &tileDef{name: "Hotel suite", w: 10, h: 3, states: []TState{TVacant, TOccupied, TVIP}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			g, col := " ", "p"
			if st != TVacant {
				g = "☺"
			}
			if st == TVIP {
				col = "y"
			}
			return r3(" ▪ ☼   ▪ ", "▀▄▄ ▬▬ "+g+"♣", "▀▀▀ ▀▀  ▀"), r3(" d y   d ", "cbb ss "+col+"l", "BBB dd  d")
		}}
	tiles[KindFastFood] = &tileDef{name: "Fast food", w: 16, h: 3, states: []TState{TOpen, TClosed, TRush}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			sign := centerText("HAMBURGER", 15)
			n := 0
			if st == TRush {
				n = 6
			} else if st == TOpen {
				n = 3
			}
			slots := []int{0, 2, 11, 13, 10, 1}
			b := []rune(rep(" ", 15))
			pb := []byte(rep(" ", 15))
			b[5], b[6], b[7], b[8], b[9] = '║', '▓', '▓', '▓', '║'
			pb[5], pb[6], pb[7], pb[8], pb[9] = 'h', 'N', 'N', 'N', 'h'
			if st != TClosed {
				b[7], pb[7] = '☺', 'p'
			}
			for i := 0; i < n; i++ {
				b[slots[i]], pb[slots[i]] = '☺', 'p'
			}
			sp := rep("n", 15)
			if st == TClosed {
				sp = rep("k", 15)
			}
			return r3(sign, string(b), " ╨ ╨ ▀▀▀▀▀ ╨ ╨ "), r3(sp, string(pb), " h h NNNNN h h ")
		}}
	tiles[KindRestaurant] = &tileDef{name: "Restaurant", w: 24, h: 3, states: []TState{TOpen, TClosed}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			sign := "☼ " + centerText("STEAK HOUSE", 19) + " ☼"
			if st == TOpen {
				return r3(sign, "☺╥☺  ☺╥☺  ☺╥☺  ☺╥☺  ☺╥☺", " ╨    ╨    ╨    ╨    ╨ "),
					r3("y"+rep("R", 21)+"y", "php  php  php  php  php", " h    h    h    h    h ")
			}
			return r3(sign, " ╥    ╥    ╥    ╥    ╥ ", " ╨    ╨    ╨    ╨    ╨ "),
				r3("k"+rep("u", 21)+"k", "khkkkhkkkkhkkkkhkkkkhkk", " k    k    k    k    k ")
		}}
	tiles[KindRetail] = &tileDef{name: "Retail shop", w: 12, h: 3, states: []TState{TOpen, TClosed}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			if st == TOpen {
				return r3(centerText("BOOK STORE", 11), "▓▓ ☺ ▓▓ ☺ ▓", "▀▀   ▀▀   ▀"), r3(rep("T", 11), "dd p dd p d", "dd   dd   d")
			}
			return r3(centerText("BOOK STORE", 11), "▓▓   ▓▓   ▓", "▀▀   ▀▀   ▀"), r3(rep("k", 11), "kk   kk   k", "kk   kk   k")
		}}
	tiles[KindCinema] = &tileDef{name: "Cinema", w: 31, h: 6, states: []TState{TShowing, TIdle}, period: 3,
		draw: func(st TState, t int) ([]string, []string) {
			show := st == TShowing
			scr, sc := "░", "k"
			if show {
				scr, sc = "█", "H"
				if t%3 == 0 {
					scr = "▓"
				}
			}
			seats, sp := "▄ ▄ ▄ ▄ ▄ ▄ ▄ ▄ ▄ ▄ ▄ ▄ ", "u u u u u u u u u u u u "
			if show {
				seats, sp = "▄☺▄☺▄☺▄☺▄☺▄☺▄☺▄☺▄☺▄☺▄☺▄☺", "uhuhuhuhuhuhuhuhuhuhuhuh"
			}
			stairs := []string{"▄█▀   ", "  ▄█▀ ", "    ▄█"}
			e1, e2 := "      ", "      "
			if show {
				e1, e2 = "☺  ☺  ", " ☺    "
			}
			rows := []string{
				"╔" + rep("═", 22) + "╗" + "CINEMA",
				"║" + rep(scr, 22) + "║" + e1,
				"╚" + rep("═", 22) + "╝" + e2,
				seats + stairs[2], seats + stairs[1], seats + stairs[0],
			}
			paint := []string{
				rep("h", 24) + rep("R", 6),
				"h" + rep(sc, 22) + "h" + "pppppp",
				rep("h", 24) + "pppppp",
				sp + "hhhhhh", sp + "hhhhhh", sp + "hhhhhh",
			}
			return rows, paint
		}}
	tiles[KindPartyHall] = &tileDef{name: "Party hall", w: 24, h: 6, states: []TState{TParty, TIdle}, period: 2,
		draw: func(st TState, t int) ([]string, []string) {
			on := st == TParty
			d1 := " ☺☺ ♪ ☺ ☺ ♫  ☺☺ ♪  ☺"
			if t%2 == 1 {
				d1 = "☺☺ ♪  ☺ ☺  ♫ ☺☺  ♪ ☺ "
			}
			row2, row4 := rep(" ", 23), "  ╨    ╨    ╨    ╨    "
			lamp, sign, floor := "k", "k", "k"
			if on {
				row2, row4 = d1, " ☺╥☺  ☺╥☺  ☺╥☺  ☺╥☺  "
				lamp, sign, floor = "y", "v", "d"
			}
			return []string{"  ☼     ☼     ☼     ☼  ", centerText("═ PARTY HALL ═", 23), row2, "  ╥    ╥    ╥    ╥    ", row4, rep("▀", 23)},
				[]string{rep(lamp, 23), rep(sign, 23), padEnd("pyypppyyppppypp", 23, "p"), rep("h", 23), "phpphpphpphpphpphpphpph", rep(floor, 23)}
		}}
	tiles[KindHousekeeping] = &tileDef{name: "Housekeeping", w: 15, h: 3, states: []TState{TDay}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			return r3(centerText("HOUSEKEEPING", 14), "☺▬ ☺▬ ☺▬ ☺▬  ", " ◙  ◙  ◙  ◙   "), r3(rep("S", 14), "ph ph ph ph   ", " h  h  h  h    ")
		}}
	tiles[KindSecurity] = &tileDef{name: "Security office", w: 16, h: 3, states: []TState{TDay}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			return r3("■ ■ ■ SECURITY ", "☺ ☺ ☺ ☺ ☺ ☺  ▬▬", rep("▀", 15)), r3("A A A QQQQQQQQ ", "p p p p p p  hh", rep("k", 15))
		}}
	tiles[KindMedical] = &tileDef{name: "Medical center", w: 26, h: 3, states: []TState{TDay}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			return r3(centerText("+ MEDICAL CENTER +", 25), "▀▄▄ ☺  ▀▄▄ ☺  ▀▄▄ ☺  ▀▄▄ ", "▀▀▀    ▀▀▀    ▀▀▀    ▀▀▀ "),
				r3(rep("S", 25), "cbb p  cbb p  cbb p  cbb ", "BBB    BBB    BBB    BBB ")
		}}
	tiles[KindParkingRamp] = &tileDef{name: "Parking ramp", w: 15, h: 3, states: []TState{TDay}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			return r3("P         ▄▄▀▀", "     ▄▄▀▀     ", "▄▄▀▀          "), r3("Q         hhhh", "     hhhh     ", "hhhh          ")
		}}
	// Parking: the mock cycles the car color with t (it strobes at 4 fps);
	// here "t" is a per-space variant the renderer derives from X, so each
	// car keeps one color.
	tiles[KindParking] = &tileDef{name: "Parking space", w: 4, h: 3, states: []TState{TCar, TEmpty}, period: 4,
		draw: func(st TState, t int) ([]string, []string) {
			col := string("rQyg"[t%4])
			if st == TCar {
				return r3("   ", "▄█▄", "◙ ◙"), r3("   ", col+col+col, "h h")
			}
			return r3("   ", "   ", "┴ ┴"), r3("   ", "   ", "k k")
		}}
	tiles[KindMetro] = &tileDef{name: "Metro station", w: 30, h: 9, states: []TState{TTrain, TEmpty}, period: 3,
		draw: func(st TState, t int) ([]string, []string) {
			tr := []rune("▐" + rep("█", 10) + "▌")
			pos := 8 + (t*2)%6
			tunnel := []rune(rep(" ", 29))
			tp := []byte(rep(" ", 29))
			if st == TTrain {
				for i := range tr {
					if pos+i < 29 {
						tunnel[pos+i], tp[pos+i] = tr[i], 'h'
					}
				}
			}
			return []string{centerText("══ METRO ══", 29), "  ☺   ╫ ╫ ╫   ☺   ☺      ", "      ╨ ╨ ╨                ",
					"  ☺  ☺    ☺   ☺    ☺  ☺    ", rep("▀", 29), rep(" ", 29), string(tunnel), rep("═", 29), rep(" ", 29)},
				[]string{rep("S", 29), "  p   hhhhh   p   p      ", "      hhhhh                ", "  p  p    p   p    p  p    ",
					rep("h", 29), rep(" ", 29), string(tp), rep("k", 29), rep(" ", 29)}
		}}
	tiles[KindCathedral] = &tileDef{name: "Cathedral", w: 28, h: 12, states: []TState{TWedding, TIdle}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			on := st == TWedding
			r := []string{
				"             ╬            ",
				"            ▄█▄           ",
				"           ▄███▄          ",
				"          ███████          ",
				"   ∩  ∩   █ ▓ █   ∩  ∩   ",
				"   ▓  ▓   █ ▓ █   ▓  ▓   ",
				"   ▓  ▓   █████   ▓  ▓   ",
				" ∩∩∩∩∩∩∩∩∩ ∩ ∩∩∩∩∩∩∩∩∩ ",
			}
			if on {
				r = append(r, " ☺ ☺ ☺ ☺  ♪ ☺☺ ♪  ☺ ☺ ☺ ☺ ", " ☺ ☺ ☺ ☺   ☺  ☺   ☺ ☺ ☺ ☺ ", " ☺ ☺ ☺ ☺    ▄▄    ☺ ☺ ☺ ☺ ")
			} else {
				r = append(r, rep(" ", 27), rep(" ", 27), "             ▄▄            ")
			}
			r = append(r, rep("▀", 27))
			p := make([]string, len(r))
			for i, row := range r {
				var b strings.Builder
				for _, ch := range row {
					switch ch {
					case '▓':
						b.WriteByte("Qryg"[(i*3)%4])
					case '☺':
						b.WriteByte('p')
					case '♪', '╬':
						b.WriteByte('y')
					default:
						b.WriteByte('H')
					}
				}
				p[i] = b.String()
			}
			return r, p
		}}
	tiles[KindRecycling] = &tileDef{name: "Recycling center", w: 25, h: 6, states: []TState{TDay}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			return []string{centerText("RECYCLING", 24), " ▓▓  ▓▓  ▓▓  ▓▓  ▓▓   ", " ▀▀  ▀▀  ▀▀  ▀▀  ▀▀   ", "           ═════════════", "  ▄████▄    ▓ ▓ ▓        ", "  ◙    ◙    ▀ ▀ ▀        "},
				[]string{rep("S", 24), " gg  gg  gg  gg  gg   ", " GG  GG  GG  GG  GG   ", "           kkkkkkkkkkkkk", "  hhhhhh    g g g        ", "  h    h    G G G        "}
		}}
	stairRows := func() []string {
		return []string{"      ▄█", "     ▄█▀", "    ▄█▀ ", "   ▄█▀  ", "  ▄█▀   ", "▄█▀     "}
	}
	tiles[KindStairs] = &tileDef{name: "Stairs", w: 8, h: 6, noWall: true, states: []TState{TIdle, TUsed}, period: 6,
		draw: func(st TState, t int) ([]string, []string) {
			r := stairRows()
			p := make([]string, len(r))
			for i, x := range r {
				var b strings.Builder
				for _, ch := range x {
					if ch == ' ' {
						b.WriteByte(' ')
					} else {
						b.WriteByte('h')
					}
				}
				p[i] = b.String()
			}
			if st == TUsed {
				i := 5 - t%6
				pos := 6 - i - 1
				if pos < 0 {
					pos = 0
				}
				r[i] = setRune(r[i], pos, '☺')
				p[i] = p[i][:pos] + "p" + p[i][pos+1:]
			}
			return r, p
		}}
	tiles[KindEscalator] = &tileDef{name: "Escalator", w: 8, h: 6, noWall: true, states: []TState{TRun}, period: 6,
		draw: func(st TState, t int) ([]string, []string) {
			r := []string{"      ▄≡", "     ▄≡▀", "    ▄≡▀ ", "   ▄≡▀  ", "  ▄≡▀   ", "▄≡▀     "}
			p := make([]string, len(r))
			for i, x := range r {
				var b strings.Builder
				for _, ch := range x {
					switch ch {
					case '≡':
						b.WriteByte('A')
					case '▄', '▀':
						b.WriteByte('h')
					default:
						b.WriteByte(' ')
					}
				}
				p[i] = b.String()
			}
			i := 5 - t%6
			pos := 6 - i
			r[i] = setRune(r[i], pos, '☻')
			p[i] = p[i][:pos] + "y" + p[i][pos+1:]
			return r, p
		}}
	elev := &tileDef{name: "Standard elevator", w: 4, h: 3, shaft: true, states: []TState{TOpen, TMoving, TEmpty}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			switch st {
			case TEmpty:
				return r3("··", "··", "··"), r3("kk", "kk", "kk")
			case TMoving:
				return r3("▄▄", "▐▌", "▀▀"), r3("hh", "HH", "hh")
			}
			return r3("▄▄", "☺☺", "▀▀"), r3("hh", "pp", "hh")
		}}
	tiles[KindElevator] = elev
	svc := *elev
	svc.name = "Service elevator"
	tiles[KindService] = &svc
	tiles[KindExpress] = &tileDef{name: "Express elevator", w: 6, h: 3, shaft: true, states: []TState{TOpen, TMoving}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			if st == TMoving {
				return r3("▄▄▄▄", "▐██▌", "▀▀▀▀"), r3("hhhh", "HHHH", "hhhh")
			}
			return r3("▄▄▄▄", "☺☺☺☺", "▀▀▀▀"), r3("hhhh", "pppp", "hhhh")
		}}
	tiles[KindLobby] = &tileDef{name: "Lobby", w: 16, h: 3, noWall: true, states: []TState{TDay, TNight}, period: 1,
		draw: func(st TState, t int) ([]string, []string) {
			l := "o"
			if st == TNight {
				l = "y"
			}
			return r3("║   ☼       ☼   ", "║  ☺   ▬▬▬   ☺ ", "║  ╨   ▀▀▀   ∩ "),
				r3("H   "+l+"       "+l+"   ", "H  o   ooo   o ", "H  o   ooo   o ")
		}}

	for k, td := range tiles {
		if td == nil {
			continue
		}
		for _, st := range td.states {
			var frames [animFrames]art
			for f := 0; f < animFrames; f++ {
				if td.period <= 1 && f > 0 {
					frames[f] = frames[0]
					continue
				}
				rows, paint := td.draw(st, f)
				frames[f] = buildArt(td, rows, paint)
			}
			tileCache[k][st] = &frames
		}
	}
}

// buildArt applies the mock's blit() rules to one drawn frame.
func buildArt(td *tileDef, rows, paint []string) art {
	aw := td.w - 1
	a := art{w: td.w, h: td.h, cells: make([]artCell, td.w*td.h)}
	for r := 0; r < td.h; r++ {
		var row, pr string
		if r < len(rows) {
			row = CP(rows[r])
		}
		if r < len(paint) {
			pr = paint[r]
		}
		for i := 0; i < aw; i++ {
			ch, pk := byte(' '), byte(' ')
			if i < len(row) {
				ch = row[i]
			}
			if i < len(pr) {
				pk = pr[i]
			}
			if pk == '.' || (ch == ' ' && pk == ' ') {
				continue
			}
			col := pal[pk&127]
			a.cells[r*td.w+i] = artCell{c: screen.Cell{Ch: ch, Fg: col[0], Bg: col[1]}, on: true}
		}
		if !td.noWall && !td.shaft {
			a.cells[r*td.w+aw] = artCell{c: screen.Cell{Ch: gWallD, Fg: wallFg, Bg: 0}, on: true}
		}
	}
	return a
}

// canvas is a clipped cell sink (the App's viewport, or a test grid).
type canvas interface {
	set(x, y int, c screen.Cell)
}

// TileSize returns the tile footprint (units, rows) for a kind.
func TileSize(k Kind) (w, h int) {
	if td := tiles[k]; td != nil {
		return td.w, td.h
	}
	return 0, 0
}

// Blit draws the tile for kind k in state st at animation frame `frame` with
// its top-left at (x, y). It is the single drawing entry for room art.
// TBurning / TBurned are procedural and work for any kind.
func Blit(dst canvas, x, y int, k Kind, st TState, frame int) {
	td := tiles[k]
	if td == nil {
		return
	}
	if st == TBurning || st == TBurned {
		blitFire(dst, x, y, td.w, td.h, st == TBurning, frame, !td.noWall && !td.shaft)
		return
	}
	fr := tileCache[k][st]
	if fr == nil {
		fr = tileCache[k][td.states[0]]
	}
	if frame < 0 {
		frame = -frame
	}
	a := &fr[frame%animFrames]
	for r := 0; r < a.h; r++ {
		for i := 0; i < a.w; i++ {
			if c := a.cells[r*a.w+i]; c.on {
				dst.set(x+i, y+r, c.c)
			}
		}
	}
}

// blitFire: burning cells flicker red/yellow and climb; burned cells are a
// charred gray shade.
func blitFire(dst canvas, x, y, w, h int, burning bool, frame int, wall bool) {
	for r := 0; r < h; r++ {
		for i := 0; i < w-1; i++ {
			if !burning {
				ch := gLight
				if (i+r)%3 == 0 {
					ch = gMedium
				}
				dst.set(x+i, y+r, screen.Cell{Ch: ch, Fg: 8, Bg: 0})
				continue
			}
			v := (i*7 + r*3 + frame*5) % 11
			c := screen.Cell{Ch: gDark, Fg: 12, Bg: 4}
			switch {
			case v < 3:
				c = screen.Cell{Ch: gTriUp, Fg: 14, Bg: 4}
			case v < 6:
				c = screen.Cell{Ch: gMedium, Fg: 14, Bg: 4}
			case v < 8 && r%3 == 0:
				c = screen.Cell{Ch: gLowHalf, Fg: 12, Bg: 0}
			}
			dst.set(x+i, y+r, c)
		}
		if wall {
			dst.set(x+w-1, y+r, screen.Cell{Ch: gWallD, Fg: wallFg, Bg: 0})
		}
	}
}

// previewRow draws row `row` of a tile without its wall, clipped to maxw
// columns (palette preview). gray forces locked colors.
func previewRow(dst canvas, x, y int, k Kind, row, maxw int, gray bool, frame int) {
	td := tiles[k]
	if td == nil || row >= td.h {
		return
	}
	fr := tileCache[k][td.states[0]]
	a := &fr[frame%animFrames]
	for i := 0; i < a.w-1 && i < maxw; i++ {
		c := a.cells[row*a.w+i]
		if !c.on || c.c.Ch == ' ' {
			continue
		}
		if gray {
			c.c.Fg, c.c.Bg = 8, 0
		}
		dst.set(x+i, y, c.c)
	}
}

// tileStateFor picks the tile state for an item.
func tileStateFor(it *Item) TState {
	switch it.State {
	case StateBurning:
		return TBurning
	case StateBurned:
		return TBurned
	}
	switch it.Kind {
	case KindOffice:
		if it.State == StateVacant {
			return TVacant
		}
		if it.Lit {
			return TNight
		}
		return TOccupied
	case KindCondo:
		if it.State == StateForSale || it.State == StateVacant {
			return TForSale
		}
		if it.Lit {
			return TNight
		}
		return TSold
	case KindHotelSingle:
		switch it.State {
		case StateOccupied:
			return TOccupied
		case StateAsleep:
			return TAsleep
		case StateDirty:
			return TDirty
		case StateInfested:
			return TInfested
		}
		return TVacant
	case KindHotelTwin:
		switch it.State {
		case StateAsleep:
			return TAsleep
		case StateDirty, StateInfested:
			return TDirty
		}
		return TVacant
	case KindHotelSuite:
		switch it.State {
		case StateVIP:
			return TVIP
		case StateOccupied, StateAsleep:
			return TOccupied
		}
		return TVacant
	case KindFastFood:
		switch it.State {
		case StateClosed:
			return TClosed
		case StateBusy:
			return TRush
		}
		return TOpen
	case KindRestaurant, KindRetail:
		if it.State == StateClosed {
			return TClosed
		}
		return TOpen
	case KindCinema:
		if it.State == StateActive {
			return TShowing
		}
		return TIdle
	case KindPartyHall:
		if it.State == StateActive {
			return TParty
		}
		return TIdle
	case KindMetro:
		if it.State == StateActive {
			return TTrain
		}
		return TEmpty
	case KindCathedral:
		if it.State == StateActive {
			return TWedding
		}
		return TIdle
	case KindStairs:
		if it.State == StateActive {
			return TUsed
		}
		return TIdle
	case KindParking:
		if it.State == StateOccupied {
			return TCar
		}
		return TEmpty
	case KindEscalator:
		return TRun
	}
	return TDay
}
