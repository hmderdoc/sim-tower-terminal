# SimTower (Maxis / OPeNBooK, 1994) - Gameplay Mechanics Spec for a 1:1 Clone

Compiled 2026-10-19 from online sources. Raw captures of every source used are in
`/tmp/claude-1001/-sbbs/17ff2604-de77-4ac5-a663-f0f536c1adc8/scratchpad/research/web/` (index: `SOURCES.txt` there).
OpenSkyscraper was deliberately NOT consulted.

## 0. Sources and how much to trust them

| Tag | Source | URL | Reliability notes |
|---|---|---|---|
| **EXE** | phulin/tower-together `specs/` - reverse-engineered from SIMTOWER.EXE with Ghidra, then validated tick-for-tick against the original binary running in a Unicorn emulator (state-trace parity tests). MIT. | https://github.com/phulin/tower-together/tree/main/specs (method: https://phulin.me/blog/simtower) | **Highest for numbers, timings, formulas** (values pulled from the EXE's YEN/tuning resources). Caveats: written by an LLM-driven process; some *labels* are wrong or self-contradictory (listed in section 10). The analysed binary appears to be the **v1.1-patched** build (elevator capacity 21, not 17). |
| **EXE-src** | Same repo, `apps/worker/src/sim/{progression,resources,ledger}.ts` | https://github.com/phulin/tower-together/tree/main/apps/worker/src/sim | Used only for star thresholds and YEN tables cited from the binary. Its UI cost/width table has obvious errors (e.g. party hall 27 wide, metro $45k) and is NOT used. |
| **MAN** | SimTower PC User's Manual (UK print, OCR) | https://archive.org/details/SimTower_-_Manual_-_PC | Authoritative for UI/intent. Its own "Inside the Simulation" chapter says its numbers are "just for modelling purposes"; FAQs say "many of the figures are just plain wrong" (printed before release). |
| **FAN** | SimTower Fandom wiki | https://simtower.fandom.com/wiki/ (pages: Segment, Object, Elevator, Hotel, Condo, Office, Fast_Food, Restaurant, Retail_Shop, Cinema, Party_Hall, Cathedral, Lobby, Floor, Escalator, Evaluation, Star_rating, Transit, Map_window, Query_Tool, Metro_station) | Small wiki, mostly player-observed. Good for sizes. Some errors. |
| **GF2168** | GameFAQs "SimTower FAQ" (Brad Stuart, 1995; README-derived, incl. Mac 1.2) | https://web.archive.org/web/2010/http://www.gamefaqs.com/pc/565191-simtower/faqs/2168 | Good; numbers from the shipped README/updater README. |
| **GF28905** | GameFAQs FAQ/Walkthrough, Kiwizoid v2.02 | .../faqs/28905 (same archive prefix) | Good player observations. |
| **GF6905** | GameFAQs Unofficial FAQ, Steve Novicki | .../faqs/6905 | Useful daily timetable. |
| **GF21043, GF7772, GF44386** | Other GameFAQs FAQs | .../faqs/21043, /7772, /44386 | Corroboration only. |
| **RO** | Relentless Optimizer "SimTower Reference" (PC 1.0, "double-checked") | https://relentlessoptimizer.com/gaming/2021/03/13/simtower-reference/ | Careful measurements; a few unlock-star errors. |
| **PC** | Patrick Coston SimTower FAQ | http://patcoston.com/st/faq.aspx | Mirror of GF2168 content. |
| **WP** | Wikipedia "SimTower" | https://en.wikipedia.org/wiki/SimTower | General only. |

Conventions: **money in dollars**. Internally the EXE stores cash in units of $100 (Finance window says "multiply all figures by 100"). A **quarter = 3 days** (WD, WD, WE), so every "per quarter" figure below is "per 3-day cycle". A **segment/tile** = one Floor-tool width.

---

## 1. Tower geometry and construction rules

| Rule | Value | Source |
|---|---|---|
| World width | **375 segments** (horizontal placement capped at 375 tiles) | FAN Segment; EXE COMMANDS.md |
| Floors above ground | **100** (1F..100F). Cathedral stack sits on top (see Cathedral) | MAN p.24 "limited to 100 storeys, 10 underground"; EXE |
| Basements | **10** (B1..B10). WP says "nine" (because the metro tunnel consumes B10 in practice) | MAN; EXE DATA-MODEL (logical floors -10..-1); WP |
| Internal floor index (EXE) | raw 0..119; raw 10 = 1F lobby; raw 0 = B10; raw 109 = 100F. Display: above ground `F = raw - 9`; basement `B = 10 - raw` | EXE DATA-MODEL.md |
| Floor tile cost | **$500 per segment** | MAN; FAN Floor; GF2168; EXE ECONOMY (type 0x00 = $500) |
| Auto-floor | Placing a room on unfloored space also charges the floor under it (e.g. office on bare air = $40,000 + 9 x $500 = $44,500). Info bar distinguishes "Not enough money for construction" vs "Not enough money to build floor". If you click Floor some distance from the edge, it fills the gap. | MAN p.43, p.46; FAN Floor |
| Width rule above ground | A floor can never be wider than the floor below it; cannot build wider than the lobby (above or below ground). No gaps: all construction grows from one foundation; two lobbies merge. | MAN p.24, p.30, p.46; EXE COMMANDS (missing support -> reject 0x03, overflow -> 0x06) |
| Basement rule | A basement floor needs the floor above to exist, must stay within the basement entrance footprint, and a new span on an empty basement floor must overlap the occupied range of the floor above. | EXE COMMANDS.md |
| Lobby (1F) | Must be placed first, on 1F. Draggable. Cannot be removed. Edges (emergency stairs + entry canopy) can't be built on. | MAN p.10, p.45 |
| Sky lobbies | Lobby may also be placed on **15, 30, 45, 60, 75, 90** only. Max 10 lobby/concourse objects (EXE). | MAN p.45; FAN Lobby/Skylobby; EXE COMMANDS |
| Multi-story lobby | Hold **Ctrl** (2-story) or **Ctrl+Shift** (3-story) on the very first lobby click; locks `lobby_height` 1/2/3 for the game. Upper lobby floors cost floor-premium x lobby_height. Elevators can't stop on the upper lobby floors. Taller lobby reduces boarding stress at 1F by 25 (2-story) / 50 (3-story) ticks. | FAN Lobby; EXE ECONOMY/LOBBY/PEOPLE |
| "Double funds" quirk | First click as a lobby at B10 column 0 while cash is the starting value doubles funds (FAQ); EXE has an explicit guard/refund around exactly this click. Treat as a bug to optionally reproduce. | GF21043; EXE COMMANDS |
| Max height of anything but cathedral | Non-cathedral families rejected above 100F; elevator top floor <= 100F | EXE COMMANDS |

**What can go underground** (FAN infobox "can be placed underground" + EXE floor-class rules):

| Underground OK | Underground only | Above ground only |
|---|---|---|
| Restaurant, Fast Food, Retail Shop, Cinema, Party Hall, Security, Medical Center, Stairs, Escalator, all elevators, Floor | Parking Space, Parking Ramp, Recycling Center, Metro Station | Office, Condo, all Hotel rooms, Housekeeping (FAN Hotel), Lobby (1F/sky only), Cathedral (top) |

Notes: once a Metro exists, cinema / recycling / party hall cannot be placed below the metro's top floor, and generic items cannot go below `metro_floor - 1` (EXE METRO). Nothing may be built under the metro or on its bottom (tunnel) floor (MAN p.51, FAN). Kiwizoid: never put floor/elevator tiles on B10 or the metro can't be placed (GF28905). EXE text claiming "security office is basement-only" and "cinema/party hall forbidden underground" contradicts every player source; treat as EXE-spec mislabels (see section 10).

---

## 2. Items (master table)

Sources per column: **Size** FAN Segment (https://simtower.fandom.com/wiki/Segment) + EXE; **Cost** EXE ECONOMY.md construction table (binary build-label strings) cross-checked with GF2168/GF28905/RO; **Upkeep** EXE ECONOMY.md + YEN #1002 (EXE-src resources.ts) + GF2168/RO; **Star** FAN Star_rating + GF2168/PC; **Pop** FAN/GF2168/EXE.

| Item | W x H (segments x floors) | Build cost | Upkeep / quarter | Income | Population counted | Unlock star | Removable |
|---|---|---|---|---|---|---|---|
| Floor | 1 x 1 | $500/seg | 0 | - | 0 | 1 | No |
| Lobby | 1 x 1 per seg (tool lays 4) | **$5,000/seg** (+$500 floor) - FAN says $1,250/seg, see note | **$0 (1-2 stars), $300/seg (3 stars), $1,000/seg (4+ stars)** | - | 0 | 1 | No |
| Stairs | 8 x 2 | $5,000 | $0 | - | - | 1 | Yes |
| Escalator | 8 x 2 | $20,000 | $5,000 (EXE scales by `(unit_count/2)+1`) | - | - | 3 | Yes |
| Standard elevator | shaft 4 wide | $200,000 shaft + **$80,000/car** | $10,000 per car | - | - | 1 | Yes |
| Service elevator | shaft 4 wide | $100,000 shaft + $50,000/car | $10,000 per car | - | - | 2 | Yes |
| Express elevator | shaft 6 wide | $400,000 shaft + $150,000/car | $20,000 per car | - | - | 3 | Yes |
| Office | 9 x 1 | $40,000 | 0 | rent $15k/$10k/$5k/$2k per quarter | 6 | 1 | Yes |
| Condo | 16 x 1 | $80,000 | 0 | one-time sale $200k/$150k/$100k/$40k | 3 | 1 | Yes (must buy back) |
| Fast Food | 16 x 1 | $100,000 | 0 | daily -$3k / +$2k / +$3k / +$5k | yesterday's customers (10-50) | 1 | Yes |
| Single hotel room | 4 x 1 | $20,000 | 0 | per stay $3,000/$2,000/$1,500/$500 | 0-1 | 2 | Yes |
| Twin hotel room | 6 x 1 | $50,000 | 0 | per stay $4,500/$3,000/$2,000/$800 | 0-2 | 3 (RO/EXE-src say 2; see note) | Yes |
| Hotel suite | 10 x 1 | $100,000 | 0 | per stay $9,000/$6,000/$4,000/$1,500 | 0-2 (EXE allocates 3 sim slots) | 3 | Yes |
| Housekeeping | 15 x 1 | $50,000 | $10,000 | - | not counted | 2 | **No** |
| Security office | 16 x 1 | $100,000 | $20,000 | - | not counted | 2 | **No** |
| Restaurant | 24 x 1 | **$200,000** (FAN says $100,000 - wrong; EXE triple-checked label "Restaurant - $200000") | 0 | daily -$6k / +$4k / +$6k / +$10k | yesterday's customers | 3 | Yes |
| Retail shop | 12 x 1 | $100,000 | 0 | rent $20k/$15k/$10k/$4k per quarter | yesterday's customers (FAN 20-50) | 3 | Yes |
| Cinema (Movie Theater) | 31 x 2 | $500,000 (+ $150k classic / $300k new movie) | 0 | daily $0/$2k/$10k/$15k by attendance | attendance | 3 | FAN: Yes; MAN: "cannot be removed" (see note) |
| Party hall | 24 x 2 | $100,000 | 0 | $20,000 per day with any attendance | 50 (FAN) | 3 | Yes |
| Medical center | 26 x 1 | $500,000 | $0 | - | not counted | 3 | GF28905 says indestructible; UNVERIFIED |
| Parking space | 4 x 1 (FAN) | $3,000 | $0 | - | 0 | 3 | Yes (EXE lists parking variants non-removable; UNVERIFIED) |
| Parking ramp | 16 x 1 | $50,000 | $10,000 | - | 0 | 3 | Yes |
| Recycling center | 25 x 2 | $500,000 | $50,000 (GF2168 says "/yr", EXE/RO say per 3-day pass) | - | 0 | 3 | Yes |
| Metro station | 30 x 3 (bottom tunnel floor spans whole width) | $1,000,000 | $100,000 | - | brings visitors | 4 | **No** |
| Cathedral | 28 x 4 (FAN/RO; EXE places a 5-slice stack) | $3,000,000 | $0 | - | 0 (FAN) | 5 | **No** |

Rent/price tiers are listed **highest to lowest**: tier 0 (High), tier 1 (**default, Average**), tier 2 (Low), tier 3 (Very Low). Source for every payout row: EXE YEN #1001 (`facility/HOTEL.md`, `OFFICE.md`, `CONDO.md`, `COMMERCIAL.md`, `ENTERTAINMENT.md`), matching GF2168 except condo tier 3 (GF2168 $50k vs EXE $40k; EXE more reliable).

Notes on conflicts in this table:
- **Lobby cost**: GF2168 ("$5000 for a section the size of the floor tool... the lobby tool actually places 4 sections") and GF28905 ("$22,000" per click = 4 x $5,000 + 4 x $500, citing the README) agree on $5,000/segment; EXE build label for type 0x18 is $5,000. FAN's $1,250/seg is likely a misreading. Use **$5,000/segment**.
- **Lobby upkeep**: EXE describes a "parking expense" routine with exactly $0/$300/$1,000 per tile by star and applied to type 0x18 (which is the Lobby family code; EXE-src maps lobby to 0x18) and skipping upper floors of a multi-story lobby. RO independently measured "Lobby maintenance is free for one and two stars, $300 per segment for three stars, and $1,000 per segment for four stars". So this is **lobby** maintenance, not parking. GF2168 also notes lobby upkeep starts at 3 stars.
- **Twin/suite unlock**: FAN, GF2168, PC, GF28905, MAN tutorial say 3 stars; RO and EXE-src say 2. Use **3** (majority, and GF2168 is README-sourced). UNVERIFIED in the binary.
- **Cinema removable**: MAN says theaters "cannot be removed"; FAN infobox says deletable. Not resolved (UNVERIFIED).
- **Standard elevator per-floor extension**: tower-together emulator trace observed $15,000 for extending a standard shaft 3 floors ($5,000/floor; $7,500/floor for express). UNVERIFIED by any player source (may be floor-tile charges).

### 2.1 Count limits

| Limit | Value | Source |
|---|---|---|
| Elevator shafts (all types combined) | 24 | MAN p.47; GF2168; EXE |
| Cars per shaft | 8 | GF2168; EXE. MAN: "eight cars maximum at the 5-star rating" implies lower per-star car caps - UNVERIFIED (MAN also says the Info Bar reports per-star limits "e.g. number of elevator cars") |
| Stairs + escalators | 64 combined | GF2168; FAN; EXE (64-slot special-link table) |
| Fast food + restaurant + retail | 512 (GF2168/RO) vs 200 active placements (EXE COMMANDS) | conflict; EXE venue record table has 512 slots, so 512 storage / 200 placement both plausible - UNVERIFIED |
| Parking spaces | 512 | GF2168; RO |
| Cinemas + party halls | 16 | GF2168; EXE (16 entertainment links) |
| Security offices | 10 | GF2168; EXE |
| Medical centers | 10 | GF2168; EXE MEDICAL |
| Metro | 1 | all |
| Cathedral | 1 | all |
| Named sims (Find Person) | 20 | GF2168 |
| Elevator queue per floor per direction | 40 people | EXE ELEVATORS |
| Per-floor placed objects | 150 slots | EXE DATA-MODEL |
| Cash cap | $99,999,999 | EXE ECONOMY |

### 2.2 Per-item rules

**Office** (EXE OFFICE.md; MAN p.48; FAN Office; GF6905)
- 6 workers. Rent collected when first occupied and then every quarter (EXE: cashflow activates at the 3-day pass, tick 2533 = 4:59 AM of the first weekday; GF6905 "5:00 AM every first weekday").
- An office opens only when a worker actually reaches it from the lobby via the route network (route result accepted), not by connectivity alone. New offices rent only on **weekdays**, during the morning commute window (EXE gate: 7:00 AM-12:59 PM; GF6905: "only sell 9:00 AM-12:59 PM on weekdays").
- Workers do not come on weekends. Lunch is at fast food (not restaurants). At 3+ stars about 1 worker in 4 ((floor+slot)%4==1) demands parking (EXE PARKING) - "Office workers demand Parking" message; at 3+ stars each worker has a 1-in-10 chance per workday of a medical trip (EXE MEDICAL).
- Workers walk up to **79 segments** horizontally before "Stairs/Elevators are far away" (FAN Office). EXE: horizontal distance penalty +30 ticks stress if distance >79, +60 if >=125.
- Leaves (rent stops, back to "For Rent") when its evaluation is red (eval 0) at the 3-day pass (EXE). MAN: "A full game day of stress levels in the red will make them vamoose."
- Noise-sensitive within 10 segments on the same floor (see 5.4).

**Condo** (EXE CONDO.md; MAN p.49, p.60; FAN Condo)
- 3 residents. Sold once at the chosen price tier: $200,000 / **$150,000 (default)** / $100,000 / $40,000 (default is ~1.9x the $80,000 build cost; max 2.5x, matching FAN).
- Price can only be changed while unsold.
- Sale fires when a resident's route to the condo succeeds (needs real transport). FAN: not bought between 9 PM and 7 AM; unconnected condos never sell (can be "banked").
- If evaluation is red at the 3-day pass, residents leave and **the full sale price is deducted** (can push cash negative). Bulldozing a sold condo also deducts the sale price (MAN).
- FAN: "Conditions are terrible" condos won't sell until bulldozed and rebuilt, or price set to minimum then back. FAN reports a bomb-related glitch where phantom sims occupy unsold condos.

**Hotel rooms** (EXE HOTEL.md; MAN p.48-49; FAN Hotel)
- Guests check in from the evening (EXE: check-in dispatch starts daypart 4 = 5:00 PM, steady until 12:00 AM) and check out in the morning (GF6905: 6:30 AM-noon; EXE: checkout route allowed dayparts 1-5, 1/12 chance in daypart 0). Income is paid **on checkout**, per stay, at the room's tier.
- A room is only re-rented after housekeeping cleans it (EXE: a guest can only be assigned once the housekeeping helper has claimed/cleaned the room). Dirty rooms show red in the Map "Hotel" view.
- **Cockroaches**: a checked-out room not cleaned for **3 consecutive days** becomes infested (EXE, checked daily at 5:00 PM); infestation spreads to adjacent hotel rooms on the same floor each day; only cure is demolition (MAN, FAN).
- Suites: guests need parking (GF2168 "one or two parking spaces per room"; MAN "occupied rooms require a parking place"). The VIP stays only in a suite.
- Guests make short evening trips to restaurants/fast food/retail (EXE family 0x21 picks one of the three types uniformly).

**Housekeeping** (FAN Hotel; GF2168; EXE HOUSEKEEPING.md)
- 6 housekeepers per room; each cleans up to ~19 rooms; one housekeeper per floor per housekeeping room. They start no new room after **4:30 PM** and go home at **5:00 PM** (FAN, GF2168); EXE: claims only succeed before tick 1500 (= 4:00 PM on the game clock).
- Use **service elevators or stairs only** (never standard/express elevators or escalators). EXE: housekeeper search is restricted to floors with `floor % 6 == home-floor class`, scans upward first then downward, picks first room in status "checked-out".
- Cannot be demolished (FAN, GF2168).

**Fast food / Restaurant** (EXE COMMERCIAL.md; FAN; GF2168; RO; MAN p.58)
- Fast food opens 10:00 AM, closes 9:00 PM and books income at 9:00 PM. Restaurant opens 5:00 PM, closes and books income at 11:00 PM (GF6905; FAN; EXE checkpoints 2000 / 2200 = 9:00 PM / 11:00 PM).
- Daily payout by that day's customer count (EXE thresholds 25 / 35 / 50):

| Customers | <25 | 25-34 | 35-49 | >=50 |
|---|---|---|---|---|
| Fast food | -$3,000 | +$2,000 | +$3,000 | +$5,000 |
| Restaurant | -$6,000 | +$4,000 | +$6,000 | +$10,000 |

  FAN/GF2168 list only three bands (-3k/+2k/+5k); RO measured $3k/day at 40-48 customers, corroborating the EXE's 4th band. A venue placed in the afternoon books a loss that day (GF2168).
- Daily capacity caps (EXE): weekday 35, weekend 50 (both); on "override" days (every 8th day, below 4 stars, until 5 PM) 25. Floor of 10. New venue starts at 10 customers. Customer count becomes tower population the next day (RO: fast food settles ~35 weekday / ~48 weekend).
- Minimum customer stay: 60 ticks (EXE). More than 39 inside -> new arrivals wait.
- Office workers lunch only at fast food; restaurants are evening only. Restaurants are not rented (no rent dial).
- Commercial (fast food, restaurant, retail) is a **noise source** for offices, hotels, condos.

**Retail shop** (EXE; FAN; GF2168; MAN p.59)
- Rented; rent $20k/$15k/$10k/$4k per quarter (EXE says recurring cashflow on the 3-day pass; GF2168 says daily - EXE more reliable; RO "$15k/qtr").
- Capacity caps 25 weekday / 30 weekend / 18 override (EXE). Evaluation is by customer count with a rent modifier (+5 / 0 / -5 / -12 by tier). Closes and stops paying when unprofitable; rent reduction brings it back.
- MAN modeling rule (not exact): A >20 customers, B 15-20, C <15.

**Cinema** (EXE ENTERTAINMENT.md; MAN p.42, p.50, p.60; FAN Cinema)
- 2 floors; internally each floor is a 7-tile stairway + 24-tile screen. Audience enters top floor, exits bottom floor (MAN).
- Two showings per day: upper half ~12:45 PM-4:00 PM, lower half ~3:00 PM-8:00 PM; income booked at **8:00 PM** (EXE ticks 1000-1500 / 1400-1900; GF6905 "1-4 PM and 5-8 PM, income at 8 PM").
- Attendance budget per half from movie pool and age (days since the movie was changed, /3): New releases 60/60/40/20, Classics 40/40/40/20 for ages 0-2/3-5/6-8/9+.
- Daily payout by total attendance: <40 $0; 40-79 $2,000; 80-99 $10,000; >=100 $15,000. Skipped on bomb/fire days.
- "Change Movie": **New $300,000** (cycles 7 new titles) or **Classic $150,000** (cycles 7 classics); resets age. No auto rotation. Indicators: "It's a Sellout!", "New Movie showing!", "Ticket Sales are average.", "Terrible Sales, change the movie." 14 titles listed in EXE ENTERTAINMENT.md / FAN Cinema.
- After the film, audiences shop within 5 floors up/down (MAN).

**Party hall** (EXE; FAN; MAN p.50)
- 2 floors, 50 guests in the afternoon (EXE: runs 1:00 PM-5:00 PM), **$20,000** booked at 5:00 PM if anyone attended (FAN "earns 20,000 at 7pm" - EXE checkpoint is 1600 = 5:00 PM, GF6905 "5:00 PM income from Party Halls"; use 5:00 PM). MAN: fills only if you have "the right number of hotel rooms" (UNVERIFIED as a hard rule). Same width as 2 retail shops.

**Parking** (EXE PARKING.md; MAN p.50; GF28905; GF2168)
- Spaces underground only; drag to lay rows. Each parking floor needs a ramp; ramps form one vertical column, the first must connect to the 1F lobby. Spaces must chain to a ramp (EXE: coverage walks left/right from the ramp across empty gaps of at most 3 tiles, stopping at any non-parking object); uncovered spaces show a red X. Need stairs/elevator into the parking floors (MAN).
- Demanded by offices (3+ stars) and suites. GF2168: one full-width row was enough for 5 stars.

**Recycling center** (EXE RECYCLING.md; MAN p.51; GF2168; GF28905)
- Below ground, 2 floors. 2nd+ centers must be placed adjacent (overlapping within floors anchor-2..anchor+1) to an existing center.
- MAN says a service elevator must reach it; GF2168 and GF28905 tested and found it is **not** required. Trucks collect once a day.
- Adequacy (only checked at 3+ stars): `load = population / number_of_centers`. Tier = 1 (<500), 2 (<1000), 3 (<1500), 4 (<2000), 5 (<2500), 6 (>=2500). Checked at 9:00 PM (allowed tier 2) and ~6:00 AM (allowed tier 5); midday reset at 5:00 PM clears adequacy. Failing the end-of-day check shows "recycling centers full". Adequacy gates 3->4 and 4->5 stars. Practical rule: **about one center per 2,500 population**.

**Security office** (MAN p.51; EXE; GF2168)
- 6 guards; they fight fires and search for bombs, moving **only on the tower's outside emergency stairs** (never elevators). Closer = more effective; spread them out. Required for 3 stars. Cannot be demolished; cannot be built after a fire/bomb starts.
- MAN: "Both terrorist attacks and fires will occur only if you have security personnel in your building" (EXE: fire needs 3+ stars, which implies security).

**Medical center** (MAN p.52; EXE MEDICAL.md; FAN Evaluation)
- Used mainly by office workers; required for 4 stars ("Medical Center demanded near Lobby"). EXE: at 3+ stars each office worker has a 1/10 chance per workday of a medical trip; if no center is reachable the daily medical flag clears and star advancement is blocked that day. No income, no upkeep. Condos also want proximity (FAN).

**Metro station** (EXE METRO.md; MAN p.51; FAN Metro_station; GF2168; GF28905)
- 3 floors, top anchor B1..B8 (so bottom on B3..B10); bottom floor is the full-width tunnel; floor must be completely clear. Ideal: B8-B10. One per tower. Required for 5 stars.
- Metro riders shop/eat only on underground floors (MAN), and work/live above ground. The "train at platform" sprite toggles randomly (EXE, cosmetic). Save before placing (v1.0 crash bug, GF2168).

**Cathedral** (EXE EVALUATION.md; MAN p.52; FAN Cathedral)
- Only at 5 stars; placed on top of floor 100. Express elevators only reach 90, so a standard elevator 90->100 is needed. Gives Tower rating via a weekend wedding (section 3).

**Stairs / escalators** (MAN p.46; FAN Escalator; GF2168; EXE ROUTING)
- Both span 2 floors (8 wide), stackable; cannot overlap elevators. Stairs can sit over any room; escalators only over lobby, restaurant, fast food, retail, cinema, party hall, empty floor (EXE allowlist also includes single hotel room).
- Max hops per trip: MAN "won't use more than four sets of stairs", "won't take escalators more than seven times"; GF2168 "three flights"; FAN "5 levels" stairs / "7 (practically 5)" escalators; RO "4 flights of stairs, 7 escalators". EXE: the local walk zone around a lobby scans up to 6 floors each way, but stops 3 floors out once a floor lacking escalator coverage is met; stress per floor 35 ticks (stairs) vs 16 (escalator). Use **4 stairs / 7 escalators** as the player-facing rule.
- Sims prefer escalators (no wait), then a free elevator.

**Elevators**: see section 6.

---

## 3. Star ratings

Thresholds (EXE-src progression.ts, binary DS:e630..e63c + hardcoded 15000): **300 / 1,000 / 5,000 / 10,000 / 15,000**, compared `population >= threshold`.

| Rating | Population | Extra requirements | Unlocks |
|---|---|---|---|
| 1 star | start | - | Lobby, Floor, Stairs, Standard Elevator, Office, Condo, Fast Food |
| 2 stars | 300 | none | Single hotel room, Housekeeping, Security, Service Elevator |
| 3 stars | 1,000 | >=1 Security office | Twin room, Suite, Retail, Restaurant, Express elevator, Escalator, Medical, Parking space + ramp, Recycling, Party hall, Cinema |
| 4 stars | 5,000 | Favorable **VIP** rating; 2+ hotel suites; recycling adequate; medical demand met (some sources add parking) | Metro station |
| 5 stars | 10,000 | Metro placed; recycling + medical still OK | Cathedral |
| TOWER | 15,000 | Cathedral + a **wedding** in it (weekend morning) | - |

Sources: FAN Star_rating (https://simtower.fandom.com/wiki/Star_rating), GF2168, PC, GF28905, GF6905, RO, EXE GAME-STATE.md + progression.ts.

Disagreements:
- Population for TOWER: PC lists 10,000; GF28905, GF21043, FAN, RO and the EXE (5th threshold 15,000 must exceed current star tier) all say **15,000**. Use 15,000.
- Parking at 4 stars: GF6905 and TV Tropes list it; GF2168/FAN/RO/EXE do not. Not a hard gate in the EXE (UNVERIFIED); parking matters through suite/office evaluation.
- "2 suites": in all player sources; not found as an explicit EXE check (the VIP can only come if suites exist). Treat as required.

EXE gate details (authoritative for timing):
- Advancement is checked every tick but only succeeds when `daypart >= 4` (**5:00 PM or later**) on a **weekday** (not WE).
- 2->3: security office placed.
- 3->4: office placed, recycling adequate flag, medical-service-ok flag (cleared on any failed medical trip that day), "office-service evaluation" passed (see VIP, section 7), route-viable flag (latches true at day start once at 3+ stars).
- 4->5: metro placed, recycling adequate, medical ok, route viable.
- TOWER (rank 6) is never reached through the normal check; only via the cathedral path.
- Star-up shows a congratulations popup; lobby art changes with rating (FAN Lobby).

**Tower award (wedding)** (EXE EVALUATION.md/PEOPLE.md; GF28905; FAN):
- Cathedral spawns 40 guests (8 per slice). On **weekend** days only, during daypart 0 (7:00-11:59 AM) they head from 1F lobby to the cathedral (1-in-12 per check after tick 80, guaranteed after tick 240 = 10:00 AM).
- Award when **all 40** have arrived before tick 800 (**12:30 PM**) and population is >= 15,000. Fails silently otherwise; retry next weekend. Guests return at 1:00 PM.
- Practical: needs an elevator chain lobby -> 90 -> 100 that can move 40 people in a morning.

---

## 4. Time

| Item | Value | Source |
|---|---|---|
| Calendar | Day = 1st WD, 2nd WD, WE; **quarter = 3 days**; **year = 4 quarters (12 days)**. Info bar shows e.g. "1st WD / 1Q / 1st Year". Game starts 1st WD, 1Q, Year 1. | MAN p.8, p.24, p.33; GF28905 |
| Ticks per day | **2,600** (`day_tick` 0..2599); 7 dayparts of 400 ticks (last = 200) | EXE TIME.md |
| Sim update | Each person is processed once per **16 ticks** (1/16 of the actor table per tick); elevators every tick | EXE TIME.md |
| Weekend flag | `day_counter % 3 == 2` | EXE |
| New game | $2,000,000 cash, 1 star, day 0, tick 2533 (~4:59 AM) | EXE; GF2168; MAN |
| Day counter | increments at tick 2300 (midnight); wraps at 11,988 | EXE |
| Speed | No speed slider: Options > **Fast Mode** (run as fast as the CPU allows) or fixed speed; Pause/Resume button. Game clock deliberately runs faster at night/dawn and slower at rush hours. Game pauses while scrolling and while using Map evaluation buttons / Finance window / Facility window. | MAN p.24, p.28, p.33, p.38, p.44 |
| Real-time per tick | UNVERIFIED (MAN: "frame" depends on machine speed; "one game time period consists of 300 [OCR: 3Q0] frames") | MAN p.57 |

**Clock mapping** (EXE TIME.md, `convert_simulation_tick_to_clock_time`; non-linear):

| Daypart | Ticks | Clock | Minutes per tick |
|---|---|---|---|
| 0 | 0-399 | 7:00 AM - 11:59 AM | 0.75 |
| 1 | 400-799 | 12:00 - 12:29 PM | 0.075 |
| 2 | 800-1199 | 12:30 - 12:59 PM | 0.075 |
| 3 | 1200-1599 | 1:00 - 4:59 PM | 0.6 |
| 4 | 1600-1999 | 5:00 - 8:59 PM | 0.6 |
| 5 | 2000-2399 | 9:00 PM - 12:59 AM | 0.6 |
| 6 | 2400-2599 | 1:00 - 6:58 AM | 1.8 |

Daily checkpoints (EXE TIME.md; clock times computed from the mapping above, matching GF6905's timetable):

| Tick | Clock | What happens |
|---|---|---|
| 0 | 7:00 AM | Start of day: rebuild routes/transfer caches and venue zone buckets; cathedral guests armed; every 8th day (day%8==4, <4 stars) "override" commercial capacity |
| 32 | ~7:24 AM | Recycling truck reset ("garbage pickup 7:00 AM" GF6905) |
| 240 | 10:00 AM | Commercial venues reopen and reseed capacity (shops/fast food open 10 AM); cinema/party budgets reseeded; **fire check (day%84==83), bomb check (day%60==59)** |
| 1000 | 12:45 PM | Cinema first showing starts |
| 1200 | 1:00 PM | Party hall starts; bomb resolves (explodes if not found - MAN "always explode at 1:00 pm"); cathedral guests return |
| 1400 / 1500 | 3:00 / 4:00 PM | Cinema second showing starts / first showing ends |
| 1600 | 5:00 PM | Hotel daily update (cockroach spread/expiry), housekeeping cutoff, party hall ends + $20k, recycling reset, override cleared; star advancement possible from now (weekdays) |
| 1900 | 8:00 PM | Cinema ends, income |
| 2000 | 9:00 PM | Fast food + retail close, fast food income, recycling check (tier 2), every 12th day maintenance notice |
| 2200 | 11:00 PM | Restaurants close, income |
| 2300 | 12:00 AM | Day counter +1 (date changes) |
| 2500 | 4:00 AM | Nightly reset of all sims |
| 2533 | 4:59 AM | **If day%3==0 (start of a quarter): ledger rollover, rent/cashflow activation, tenant move-outs/condo refunds for red evals, all quarterly maintenance charged** |
| 2566 | ~5:58 AM | Final recycling check (tier 5) |

Per-tick hooks after 10:00 AM: random "news" sound sampler (1/16), metro train toggle (1/100, 4+ stars). New Year / quarter events: Santa flies across on the 4Q weekend night (cosmetic; WP, GF28905, GF44386). No other New-Year mechanic found.

Weekday vs weekend: offices empty on WE; commercial capacity higher on WE (35->50, retail 25->30); elevator schedules have separate WD/WE settings; weddings only on WE; stars only advance on WD. Rain occasionally reduces visitors (~half fast-food traffic per RO; GF28905, GF44386) - rain is **not** described in the EXE spec, so its mechanical effect is UNVERIFIED.

---

## 5. People, stress and evaluation

### 5.1 Who comes when (EXE DEMAND.md / PEOPLE.md; GF6905)

| Population | Weekday pattern |
|---|---|
| Office workers (6/office) | Morning commute 7:00 AM-12:59 PM (worker 0 trickles from 7 AM; workers 1-5 burst around 1 PM in EXE; GF6905 "workday begins 9 AM"); lunch to fast food 12:00-4:59 PM, 16-tick dwell; leave 5:00 PM onward (1/6 chance per check 5-9 PM, all by night). None on weekends. |
| Hotel guests | Check in 5:00 PM-12:00 AM; one short trip to a restaurant/shop/fast food in the evening; check out next morning (mostly by noon). |
| Condo residents (3) | Leave in the morning for errands/shops/fast food and return; weekend staggering differs. |
| Commercial customers | Outside visitors to shops 10 AM-9 PM, restaurants 5-11 PM; destination chosen uniformly at random among venues in the visitor's 15-floor zone (`zone = max(0,(floor-9)/15)`). Metro riders only use underground venues. |
| Cinema/party/cathedral | See section 2.2 and 3. |

### 5.2 Stress (EXE PEOPLE.md / ROUTING.md; MAN p.57)
- Stress = average ticks spent per trip leg: `accumulated_elapsed / trip_count`. Each leg's elapsed time is clamped to 300.
- Components: elevator wait + ride time (measured from route start to boarding/arrival); stairs **+35 per floor**, escalator **+16 per floor**; horizontal distance to the transport >79 segments **+30**, >=125 **+60**; no route at all **+300**; full queue **+5** wait then retry; taller lobby subtracts 25/50 at 1F boarding.
- Per-person color (MAN p.57 and EXE): **black < 80, pink 80-119, red 120-300**. Shown on people waiting at elevators (MAN p.15).

### 5.3 Facility evaluation (EXE FACILITIES.md; MAN p.58; FAN Evaluation)
Score for offices, condos, hotel rooms = average stress of its occupants, then:
- rent tier modifier: High **+30**, Average 0, Low **-30**, Very Low **forces score to 0**;
- noise: **+60** if a noise source is within the radius on the **same floor** (either side);
- clamp >= 0.

| Score | Eval | Color |
|---|---|---|
| < 80 | Excellent (A) | Blue |
| 80 - 149 (80 - 199 at 4+ stars) | Good (B) | Yellow |
| >= 150 (>= 200 at 4+ stars) | Terrible (C) | Red |

MAN's modelling version: quality = 300 - avg stress; A if avg <100, B 100-150, C >150; A -> "inhabitants bring a friend" (fills vacancies), B stay, C leave "at the end of the rest period". Eval bar ending exactly on a divider counts as the higher grade (FAN).

Commercial (fast food, restaurant, retail) evaluation is by customer count, not stress (bar shows the number of customers). Rooms with no evaluation: cathedral, cinema, housekeeping, lobby, medical, metro, parking, party hall, recycling, security (FAN Evaluation).

Consequences: office leaves / condo refunds / retail closes when eval is red at the quarterly pass (tick 2533 when day%3==0). FAN Evaluation also lists trash (insufficient recycling lowers all evals) and service proximity (condos want medical, public spaces want security, hotels want public spaces, hotels+offices want parking) - these proximity effects are not described in the EXE spec: UNVERIFIED in magnitude. MAN: complaints spread by rumor to neighbors ("slums").

### 5.4 Noise (EXE FACILITIES.md; RO; MAN p.25; FAN)

| Evaluated | Radius (same floor) | Noise sources |
|---|---|---|
| Office | 10 segments | restaurant, retail, fast food, cinema, party hall |
| Hotel room | 20 segments | restaurant, **office**, retail, fast food, cinema, party hall |
| Condo | 30 segments | **hotel rooms**, restaurant, office, retail, fast food, cinema, party hall |

RO measured "at least 11 empty segments between fast food and office; hotel rooms require 21 segments from an office" - matches the EXE radii exactly. MAN and FAN say noise also comes through floors/ceilings; the EXE search is same-floor only. Prefer EXE (corroborated by RO); vertical noise UNVERIFIED. Offices never mind other offices or hotels; hotels ignore other hotels and condos.

### 5.5 Routing limits
- **One transfer only**: a sim changes elevators at most once per trip, and only at a lobby/sky lobby (MAN p.47, p.56; GF2168; GF6905). FAN Transit: at most 2 elevators, or 1 elevator + 1 stair/escalator run, each way. EXE: routing is one leg at a time with transfers only at transfer-group floors (lobbies with carriers), first match wins.
- Route cost (EXE ROUTING): escalator `8*floors`; stairs `8*floors + 640`; direct elevator `8*floors + 640` (+1000 instead if queue full); elevator needing a transfer `8*floors + 3000` (+6000 if full). Cheapest wins; ties keep the first found (stairs/escalators scanned before elevators).
- Sims won't ride an elevator in the opposite direction to reach an express (GF2168, unverified).
- Route failure shows "Sims on floor X need a path to floor Y"-type messages, once per source floor until the layout changes (EXE OUTPUTS; GF2168).

---

## 6. Elevators

| Property | Standard | Service | Express | Source |
|---|---|---|---|---|
| Shaft width | 4 | 4 | 6 | FAN Segment; EXE |
| Cost | $200k + $80k/car | $100k + $50k/car | $400k + $150k/car | EXE; FAN; GF2168 |
| Upkeep / quarter | $10k/car | $10k/car | $20k/car | EXE; GF2168; RO |
| Capacity | 17 (v1.0) / **21 (v1.1)** | 17 / **21** | MAN 36; GF2168/FAN 34, **42 in update** | MAN p.47-48; GF2168; EXE (21/21/42) |
| Max span | **30 floors** incl. basements (MAN); EXE caps span at 31 floors | same | whole tower, stops only at B1-B10, 1F, 15, 30, 45, 60, 75, 90 | MAN p.44, p.47; EXE |
| Users | tenants/visitors | housekeepers only (also used for recycling per MAN, disputed) | tenants/visitors only | MAN; FAN; GF2168 |
| Unlock | 1 star | 2 stars | 3 stars | |

- Max 24 shafts, 8 cars per shaft. Shafts need spacing (EXE: 8 empty tiles between; elevator editor enforces >= 6 from standard, 4 from others). Can't overlap stairs/escalators.
- First car is created with the shaft; each extra car is placed by clicking a floor of the shaft with the elevator tool -> that floor becomes the car's **waiting (home) floor**, shown in red/pink on the shaft. You cannot disable service to a car's home floor.
- **Finger tool**: drag top/bottom machinery to extend/shrink; click a floor number to toggle service (warning if it would strand people). Express floors can't be toggled.
- People waiting on the **left** of the shaft are going up, **right** going down (MAN p.47).

**Speed** (EXE ELEVATORS.md motion modes; ticks per floor):

| Situation | Standard / Service | Express |
|---|---|---|
| Within 1 floor of target or previous stop | 6 ticks | 6 ticks |
| Within 2-3 floors | 3 ticks | 1 tick |
| Cruising (>=4 away) | 1 tick | 1 tick, or **3 floors per tick** when >4 floors from both ends |
| Door dwell at a stop | 5 ticks, then departure check every tick | same |

**Elevator window controls** (MAN p.34-38; FAN Elevator; EXE):
- WD / WE buttons, then per-period buttons between clock icons (EXE: 7 dayparts x WD/WE = 14 slots; FAN says 6 time frames - UNVERIFIED which the UI shows; MAN: first period 7:00 AM to noon). Each period: **Local** (default), **Express to Top** (cars only drop off going up and sweep back down picking up; morning rush), **Express to Bottom** (evening rush). Express modes are whole-shaft.
- **Waiting Car Response** (default **5** floors): a moving car within N floors answers a call instead of dispatching an idle car. EXE: choose the moving candidate if `moving_cost - idle_home_cost < N`, else the idle home car; tie -> idle car; a car already at the floor with doors closed is taken immediately. Costs: same-direction ahead = distance; behind/needs reversal = distance to turn floor + back.
- **Standard Floor Departure** (default **0** seconds): car lingers after the first boarding until `elapsed > setting x 30 ticks`, or departs when full. FAN recommends Response 1 and Departure 30 for local shafts, and express shafts serving lobby + one sky lobby with Departure 30.
- Shaft display: floor column (served = black with white number; unserved = white with red X); per-car columns: curved arrow = reversal point, black triangle = direction, triangle with "F" = full (red diamond on the Edit view), open circle = idle at home, filled circle = a passenger's destination. **Show** On/Off (draw shaft as lines to see rooms behind). **Simulate** previews the shaft a short time ahead.
- Queue: per floor per direction FIFO of 40; a car takes up to its free capacity from the current-direction queue.

---

## 7. Events

| Event | Trigger | Effect / numbers | Sources |
|---|---|---|---|
| **Terrorist bomb** | EXE: checked 10:00 AM when `day%60==59` (every 5 years), not during another bomb/fire. Random occupied floor (not lobby floors, >=4 wide). | Ransom by star: **2 stars $200,000, 3 stars $300,000, 4+ stars $1,000,000** (GF28905: "$300,000"). Pay -> over. Refuse -> security search on outside emergency stairs; if not found, explodes at **1:00 PM** and deletes a **40 x 6** block (floors bomb-2..bomb+3, tiles x-20..x+19); GF2168: "three condos wide, five floors high". Afterwards the clock jumps to 4:00 PM. No cinema/party income that day. | EXE EVENTS.md; MAN p.52; GF2168; GF28905 |
| **Fire** | EXE: 10:00 AM when `day%84==83` (every 7 years); only 3+ stars, morning, no cathedral eval running; floor >=32 wide. MAN: only with security in the tower. | Prompt: call fire rescue helicopter for **$500,000** (can decline and use Options > Call Fire Rescue later). Helicopter extinguishes; otherwise security walks the emergency stairs and burned tiles are deleted as fronts move inward from both ends; event ends when fronts meet or at 9:00 PM; clock forced to >= 4:00 PM. GF2168 says fire spreads upward between floors (EXE describes one floor) - multi-floor spread UNVERIFIED. Burned area ("Burned Area" family 0x2F) must be bulldozed/rebuilt. | EXE; MAN p.28, p.53; GF28905; GF7772 |
| **VIP visit** | At 3 stars, needs suites. EXE candidate: on days with `day%9==3` at 3 stars an evaluation is started (notification 3000); later compared against stress threshold **150**: pass -> notification 0xBBA (favorable), fail -> 0xBBB. GF6905: VIP leaves 11:00 AM the next morning and a dialog gives the verdict; failed visits repeat later. | Favorable rating is a 4-star requirement. VIP shows as a yellow/highlighted sim; the suite is lit yellow. Needs suite + parking + housekeeping + short elevator ride. EXE spec labels the evaluated entity as an "office" - mislabel; all player sources say it is a hotel suite. Exact VIP route/eval mechanics: partly UNVERIFIED. | MAN p.53; GF2168; GF6905; GF28905; EXE GAME-STATE.md |
| **Hidden treasure** | Random while building underground (MAN: "only if you have underground residences" - odd since residences can't be underground; FAQs: building underground floors). | Lump sum ~**$500,000** (GF28905, GF44386); shown in Finance "Other Income". Amount UNVERIFIED in EXE (dialog 0xBE0 exists). | MAN p.54; GF28905; GF44386; EXE OUTPUTS |
| **Santa** | 4Q weekend (Christmas) night | Sleigh crosses the sky; clickable; no effect | WP; GF28905; GF21043 |
| **Wedding** | see section 3 | Tower rating | |
| **Metro train / random news** | per tick | cosmetic sprite toggle; news sound clips; consume RNG | EXE |
| **Cockroaches** | 3 days uncleaned | room lost; spreads | section 2.2 |
| **Rain** | random | fewer visitors (UNVERIFIED mechanics) | GF28905; RO |

No boom/bust economy events or loans were found in any source: **there are no loans**; cash can go negative (e.g. condo buy-backs) and the player waits for income (GF21043: "If you're ever in debt... wait until the first weekday"). "3-star lobby decoration": FAN only says the lobby's look changes as the rating increases; no separate ornament item found (UNVERIFIED).

EXE RNG: 32-bit LCG `state = state*0x015a4e35 + 1`, output `(state>>16)&0x7fff`, seed 1 (for exact replay).

---

## 8. Finance summary

- Start: **$2,000,000** (GF2168, EXE). Cash cap $99,999,999.
- Construction: see section 2 table (+$500/segment for any new floor under it). Demolition: **no refund**, no charge (MAN p.44; EXE); bulldozing a sold condo charges its sale price.
- Income timing:

| Item | When paid | Amount |
|---|---|---|
| Office | at the start of each quarter (4:59 AM, 1st WD) while occupied; first payment when it becomes occupied | $15k / **$10k** / $5k / $2k |
| Retail | quarterly while open | $20k / **$15k** / $10k / $4k |
| Hotel | per checkout | single $3,000/**$2,000**/$1,500/$500; twin $4,500/**$3,000**/$2,000/$800; suite $9,000/**$6,000**/$4,000/$1,500 |
| Condo | once on sale; reversed on move-out | $200k / **$150k** / $100k / $40k |
| Fast food | daily 9:00 PM | -$3k / $2k / $3k / $5k |
| Restaurant | daily 11:00 PM | -$6k / $4k / $6k / $10k |
| Cinema | daily 8:00 PM | $0 / $2k / $10k / $15k |
| Party hall | daily 5:00 PM | $20k if attended |
| Treasure | random | ~$500k (UNVERIFIED) |

(bold = default tier)

- Maintenance, charged once per quarter at 4:59 AM of the 1st WD: standard car $10k, service car $10k, express car $20k, escalator $5k, stairs $0, security $20k, housekeeping $10k, recycling $50k, metro $100k, parking ramp $10k, lobby $0/$300/$1,000 per segment at <=2/3/4+ stars, medical $0, cathedral $0, parking space $0 (EXE ECONOMY + YEN #1002; GF2168; RO).
- Finance window (per current quarter, x100): Total Income, Total Maintenance, per-category detail lines, Net Revenues, Other Income (treasure), Construction Costs, Quarter's Balance (vs prior period), Total Balance (= Fund).
- RO's per-quarter yardsticks (useful sanity checks): office $10k, single $6k (3 nights x $2k), twin $9k, suite $18k, fast food ~$9k, restaurant ~$18k, party hall $60k, cinema ~$30k.

---

## 9. UI reference (MAN; FAN Map_window / Query_Tool)

**Menus**: File (New, Open, Save, Save As..., Quit); Edit (present but unused); Options (Animation > People / Effect; Sound > Elevators / Background / Events; Fast Mode; Call Fire Rescue - enabled only during a fire); Windows (Tool Bar, Info Bar, Map Window, Finance Window..., Find Person...).

**Info Bar** (movable/closable): analog clock; star row (5 slots, gold when earned; "TOWER" beyond); date "1st WD / 2nd WD / WE", quarter "1Q-4Q", year; message line (build errors, demands like "Office workers demand Parking", "Medical Center demanded near Lobby", income source notices, route failures); **Fund**; **Pop**.

**Tool Bar** (movable): row 1 Pause/Resume; row 2 **Bulldozer**, **Finger** (elevator extend/shrink + floor service toggle), **Magnifying Glass** (query; the only tool usable while paused). Then facility buttons, several with click-and-hold pop-ups that grow with star rating: Lobby -> Floor, Stairs, Escalator; Office; Condo; Fast Food; Standard Elevator -> Service, Express; Security -> Housekeeping; hotel rooms; Restaurant; Retail; Cinema; Party Hall; Parking/Ramp; Recycling; Medical; Metro; Cathedral. Cursor shows the item outline; Info Bar shows name and price. Shift+click with a new item over an existing one replaces it.

**Edit window**: side-view tower; scrollbars; Zoom box. Sky above ground, dirt below. Day/night lighting; weather. Elevator home floors shown as red numbers on shafts.

**Map window** (overview): gray tower silhouette; elevators as vertical lines - express blue, standard gray, service red; draggable view rectangle. Buttons: **Edit** (default; also resumes), **Eval** (blue excellent, yellow good, red terrible, gray not rated), **Pricing** (red high, yellow average, green low, light blue very low, gray n/a), **Hotel** (red = dirty rooms; only once hotels are available). Eval/Pricing/Hotel pause the game; clicking a colored spot scrolls the Edit window there.

**Facility window** (Magnifying Glass on a room; pauses): facility type + floor ("Office, Floor 27"); leasing status ("Occupied"/"For Rent"/"For Sale"); snapshot image; **Eval** bar (blue/yellow/red; for fast food/restaurant shows customer count); occupants box (click a person -> Tenant window); **Length** (time occupied); **Rent/Price** drop-down with 4 levels; message line ("Office neighbor is too noisy", "Stairs/Elevators are too far away", "Business is average", etc.). Fast food/restaurant show daily sales + customers. Cinema version adds current movie, showing length, income, "Change Movie" (New $300,000 / Classic $150,000).

**Tenant window**: gender/occupation or custom name; From (home facility + floor); Eval (of home facility, or destination for outsiders); Going To; Stress (blue/yellow/red); Rename (named sims turn blue everywhere; max 20).

**Find Person window**: list of named sims; Find scrolls to them with a red arrow, or says they're not in the tower.

**Elevator window**: see section 6. Clicking a moving car shows car particulars, not the schedule.

**Finance window**: see section 8.

---

## 10. Known contradictions inside the EXE spec (do not copy blindly)

These are places where the tower-together spec's *labels* conflict with all player sources; numbers around them are still useful.
1. **Medical center** "has no player-facing placement path" (MEDICAL.md) - contradicted by its own ECONOMY table ($500,000 type 0x0D), FAN (26 wide), MAN, every FAQ. Medical is buildable at 3 stars.
2. **"Security office is basement-only"**, **"cinema/party hall forbidden at floor 0/underground"** (COMMANDS/ENTERTAINMENT) - likely raw-vs-logical floor index confusion (the parking spec already admits "column 9" meant raw floor 9 = B1). Player sources: both placeable above or below ground.
3. **"Parking expense"** formula is actually **lobby** maintenance (type 0x18 = Lobby), confirmed by RO measurements.
4. **"office-service evaluation"** for 3->4 stars is almost certainly the **VIP suite visit**; the "cathedral guest arrives at the target office" wording is a mislabel.
5. Hotel suite population listed as 3 in places, 2 in the scoring code; player sources say 2.
6. Cathedral height: EXE stacks 5 slices; FAN/RO 4 floors. Visual height UNVERIFIED (may include a roof row).
7. Commercial cap 200 (EXE) vs 512 (README-based FAQs).
8. The `facility_progress_override` is described as "more generous" capacity but its caps (25/18) are lower than normal (35/25).

## 11. Items explicitly UNVERIFIED (collected)
Real-time length of a tick; per-star elevator car caps; twin/suite unlock (3 vs 2 stars); cinema removability; medical removability; vertical noise; rain effect; treasure amount and trigger; exact VIP mechanics/frequency (GF6905 "once every nine years", EXE day%9==3 -> every 9 days = 3 quarters); fire spreading between floors; parking as a hard 4-star gate; 6 vs 7 elevator schedule periods in the UI; elevator extension per-floor cost; party hall needing hotel rooms; lobby decoration/ornaments.
