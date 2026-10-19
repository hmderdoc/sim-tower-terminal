# OpenSkyscraper: Gameplay Mechanics Extraction

Sources (read-only, nothing executed):

- **OS** = `openskyscraper/` (default branch `master`, commit a287b66, 2019-04-02). This is the "restart" rewrite (SFML). All paths are relative to it unless prefixed.
- **SDL** = `oss-master-sdl/` (branch `master-sdl`, acb1830, 2011-06-17). This is the older, more feature-complete SDL engine. The OS TODO.md points to it for hotel behavior ("have a look at the implementation in the `master` branch"). `stable` (b0927d9) and `renovatio` (4b4cc64) are near-identical snapshots of the same code; SDL is the newest.
- **TDT** = `doc/simtower/TDT_format.txt`, jcranmer's reverse-engineering notes on the original SimTower save format. This is the only material in the repo that describes the original game directly.

Important caveat: OpenSkyscraper is an unfinished clone. Many of its numbers are placeholders the authors marked TODO (e.g. "TODO: Specify party hall income"). Wherever a value is a guess and not a port of SimTower data, I say so. Do not treat OS numbers as authoritative SimTower values unless TDT backs them up.

Coordinate conventions (OS): 1 cell = 8 px wide; 1 floor = 36 px tall (24 px room + 12 px ceiling). `position.y` is the bottom floor. Floor 0 = ground (1F), positive = up, negative = basements (B1 = -1). `Item.cpp:35-38`, `Item.h:43`, SDL `tower/structure.cpp:19` (`ceilingHeight(12), cellSize(8, 24+12)`).

---

## 1. Tower geometry

| Rule | Value | Source |
|---|---|---|
| Cell size | 8 x 36 px | `Item.h:43`, `Game.cpp:542` |
| Horizontal tower limit | **none in OS** (no x clamp at all) | `Game.cpp` construction code has no x bound |
| SDL world bounds | x -200..+200 (400 cells), floors -10..+104 (`recti(-200,-10,400,100+10+5)`) | SDL `tower/structure.cpp:166` |
| Original floor array | 120 floors: idx 0-9 = B10..B1, idx 10 = 1F, idx 110 = 100F, plus 9 spare | TDT:151-154 |
| Original per-floor tenant index map | `uint16 indexMap[94]` (i.e. at most 94 tenants per floor) | TDT:144 |
| Deepest basement (OS) | B9 (`y < -9` blocked), except Metro may go below | `Game.cpp:169-172` |
| Camera clamp (OS) | top 360*12 px = 120 floors, bottom -360 px = -10 floors | `Game.cpp:514` |
| Ground floor (y=0) | only lobbies (plus transport) may be built; any other facility is blocked with "Only lobbies may be built on the ground floor" | `Game.cpp:294-297` |
| Lobby floors | `y % 15 == 0` only, i.e. 1F, 16F, 31F, 46F, ... ("every 15th floor"). Sky lobbies use the `lobby/sky` bitmap, the ground lobby uses `lobby/normal` | `Game.cpp:179-183`, `Lobby.cpp:16-19` |
| Sky lobby width | limited to the width of the floor below, and must not overlap non-lobby facilities on that floor | `Game.cpp:184-199` |
| Main lobby | the y=0 lobby becomes `game->mainLobby`. All routes start/end there. Only one is allowed (assert) | `Lobby.cpp:28-30` |
| Lobby extension | building a lobby next to an existing lobby on the same floor extends it rather than creating a new one | `Game.cpp:337-365` |
| Support rule | a facility above ground needs the floor directly below to span its full width; below ground it needs the floor directly above (no overhangs) | `Game.cpp:313-328` |
| Floors | auto-created/extended under every built item; empty floor cells cost $500 each | `Game.cpp:333-335, 774-814`; `Floor.h:15` |
| Non-removable | Lobby, Floor, Metro cannot be bulldozed | `Game.cpp:394-398` |
| SDL attribute flags | FlexibleWidth, Every15thFloor, NotAboveGround, NotBelowGround, AllowedOnGround, Undestructible | SDL `items/itemdescriptor.h:81-88`; enforced at SDL `tower/structure.cpp:505-521` |
| Lobby visual style by rating (OS) | `rating==2` -> row 1, `rating>=3` -> row 2 (rating index 0 = 1 star, see section 7) | `Lobby.cpp:55-57` |
| Lobby style (SDL) | stars>=3 -> 1, stars>=4 -> 2 | SDL `tower/environment.cpp:47-53` |
| Crane decoration | sits on top of the highest floor, centered | `Decorations.cpp:68-84` |
| Fire-escape stairs | 24 px decorations drawn at both ends of every above-ground floor | `Decorations.cpp:22-56` |

---

## 2. Items: full table

OS prototypes are defined in each `Item/*.h` (`OT_ITEM_PROTOTYPE`). SDL descriptors are in `ItemDescriptor X::descriptor = {type, group, category, minRating, attributes, price, cells, minUnit}`. Sizes are width x height in cells/floors.

| Item | OS size | OS price | SDL size | SDL price | SDL attrs | Notes |
|---|---|---|---|---|---|---|
| Lobby | 4x1 (min unit) | $20,000 new | 1x1, min unit 4x1 | $5,000 | Flexible, Every15th, NotBelowGround, AllowedOnGround | OS `Lobby.h:12-16`; SDL `facilities/structure/lobby.cpp:7-16` |
| Floor | 1x1 | $500/cell | n/a | n/a | | `Floor.h:13-17` |
| Stairs | 8x2 | $5,000 | 8x2 | $5,000 | mask: (0,0,4,1)+(4,1,4,1) (diagonal) | `Stairs.h:12-16`; SDL `stairslike/stairs.cpp:7-17` |
| Escalator | 8x2 | $20,000 | 8x2 | $20,000 | same mask | `Escalator.h:12-16`; SDL `escalator.cpp:7-17` |
| Standard elevator | 4 wide | $100,000 | 4x1 | $200,000 | | `Elevator/Standard.h:11-15`; SDL `elevators/standard.cpp:7-15` |
| Service elevator | 4 wide | $80,000 | 4x1 | $100,000 | | `Elevator/Service.h:11-15`; SDL `service.cpp:7-15` |
| Express elevator | 6 wide | $1,000,000 | 6x1 | $400,000 | | `Elevator/Express.h:11-15`; SDL `express.cpp:7-15` |
| Extra elevator car | | $80,000 flat (all types) | | `getCarPrice()` = 0 (stub) | | `Game.cpp:156-157`; SDL `elevators/elevator.h:67` |
| Office | 9x1 | $40,000 | 9x1 | $40,000 | NotBelowGround | `Office.h:13-17`; SDL `office/office.cpp:7-15` |
| Condo | 16x1 | $200,000 | n/a | n/a | | `Condo.h:12-16` |
| Fast Food | 16x1 | $100,000 | 16x1 | $100,000 | | `FastFood.h:12-16`; SDL `fastfood.cpp:7-15` |
| Restaurant | 24x1 | $200,000 | 24x1 | $200,000 | | `Restaurant.h:12-16`; SDL `restaurant.cpp:7-15` |
| Shop | n/a | n/a | 12x1 | $100,000 | | SDL `entertainment/shop.cpp:7-15` (11 variants) |
| Party Hall | 24x2 | $500,000 | 24x2 | $100,000 | | `PartyHall.h:12-16`; SDL `partyhall.cpp:7-15` |
| Movie Theatre | 31x2, entrance on upper floor (`entrance_offset=1`) | $500,000 | n/a | n/a | | `Cinema.h:13-18` |
| Metro Station | 30x3, entrance/exit on top floor (offset 2) | $1,000,000 | n/a | n/a | | `Metro.h:12-18` |
| Single room | n/a (bitmap only: 32 px = **4 cells**) | | 4x1 | $20,000 | NotBelowGround; 1 guest | SDL `hotel/singleroom.cpp:7-15, 40-43`; OS `SimTowerLoader.cpp:383-390` |
| Twin/Double room | n/a (bitmap 48 px = **6 cells**) | | 6x1 | $50,000 | NotBelowGround; 2 guests | SDL `hotel/doubleroom.cpp:7-15`; `hotel.cpp:32-35` |
| Suite | n/a | | 10x1 | $100,000 | NotBelowGround; 2 guests | SDL `hotel/suite.cpp:7-15` |
| Housekeeping | n/a | | 15x1 | $50,000 | Undestructible(!); 6 housekeepers | SDL `hotel/housekeeping.cpp:9-17, 31-34` |
| Parking ramp/space, Recycling, Security, Medical, Cathedral | **not implemented** in either branch; only bitmaps loaded | | | | | OS `SimTowerLoader.cpp:418-473`; SDL `simtower-names.cpp` |

Bitmap-derived widths (OS loader, px/8 = cells): office 72 px = 9, condo 128 = 16, fast food 128 = 16, restaurant 192 = 24, single 32 = 4, double 48 = 6, party hall 192 = 24, cinema hall 192 + screen 56 = 248 px = 31, metro 240 = 30. `SimTowerLoader.cpp:346-457`, `Office.cpp:80`, `Condo.cpp:87`, `FastFood.cpp:50`, `Restaurant.cpp:50`.

Star-rating prerequisites: **none encoded.** Every SDL descriptor has `minRating = 1`. OS has no gating at all.

Limits encoded:

- One Metro (`Game.cpp:289-292`, `Metro.cpp:28-29`), and it is undeletable.
- One main lobby.
- Nothing else (no cathedral limit, since there is no cathedral).

Construction spacing (OS):

- Stairs/escalators cannot be within ±3 cells of another stairlike on the same floor, nor offset 1..7 on the floor below / -7..-1 on the floor above. `Game.cpp:204-216`
- Elevators need a 1-floor clearance above and below any other elevator, and may not intersect stairlikes (rect grown by 1 floor each way). `Game.cpp:218-228, 250-274`
- Stairlikes need the upper floor to be wide enough. `Game.cpp:230-240`
- Facilities may not overlap other facilities. Transport may overlap facilities. `Game.cpp:299-311`

Icon/ID quirk: OS `enum IconNumbers` has `ICON_ELEVATOR = 3` (`Item/Factory.h:21`), but the escalator's icon is 3 and the elevators are 4/5/6. The "ICON_STAIRS || ICON_ELEVATOR" branch at `Game.cpp:204` therefore actually handles **stairs + escalator**. Icons are toolbox bitmap indices, not SimTower tenant IDs.

### 2.1 Per-item behavior and schedules (OS)

All "hour" values are game hours (see section 3). `checkHour(h)` fires once when time crosses h (`Time.cpp:100-103`).

**Office** (`Item/Office.cpp`)

- State: `occupied`, `variant` (0-5), `lit`. `rent = 10000` (`:27`).
- Move-in: if vacant, not weekend (day != 2), 7:00 <= hour < 17:00, and `isAttractive()` (meaning a lobby route exists, `:189-192`). Every 0.002-day tick there is a 10% chance (`:87-90`). The expected wait is about 0.02 day; the code comment's "1/250th" is wrong. On move-in: deposit `+rent` is credited, population = 6 (`:91-99`).
- Rent and move-out: on **day 0 (1st WD) at 5:00**, i.e. once per quarter (see section 3). If unattractive, the tenant vacates and the deposit is **refunded** (`-rentDeposit`); otherwise `+rent` ($10,000) is credited (`:104-118`).
- Workers: 6 per office, types {Salesman, Salesman, Man, Man, Woman1, Woman2} (`:36-46`).
- Daily schedule, reset at 5:00 on weekdays (`:121-123, 227-253`): arrive U(7,8), depart U(17,19), lunch U(12,12.2) (scheduled but **not acted on**; TODO.md "Item::Office"). Stress is reset to 0 each morning.
- Salesmen: on arrival they schedule a trip out at now + U(0.01,0.02) day, and a return between 13:00 and 15:00 that day (`:202-214`).
- Lights: on during weekdays 7-17, or whenever anyone is inside (`:179-183`).
- Arriving at the office halves the person's stress (`:199`).
- Bug: all Worker priority-queue comparators compare `a` with `a` (`Office.h:59-83`), so ordering is undefined.

**Condo** (`Item/Condo.cpp`)

- `rent = 5000` (`:20`). Same move-in rule as Office (weekday 7-17, 0.002-day ticks, 10%) with a deposit (`:93-104`).
- Same day-0 5:00 rent/vacate logic (`:108-122`). This is NOT SimTower's sell-once condo model; it is a placeholder.
- Occupants (`:208-256`):
  - Adults: `randi(1,4)` (inclusive), gender biased toward male.
  - Kids: `randi(0, min(2*adults, 6))`. Max total is 10.
  - Adults leave U(7.5,9.5) and return U(17.5,19.5).
  - Kids leave 7.5 and return 15.5 or 17.5.
  - Daily jitter U(-0.1,+0.3) h is regenerated at 3:00 (`:124-126, 140-154`).
- Bugs: `actualReturnTime`/`actualDepartureTime` are swapped (`:279-287`), and the departure uses the lobby->condo route (`:163`). Movement is effectively broken.
- Lighting: night if <7 or >22, daytime 7-19, lit 19-22 (`:184-200`).

**Fast Food** (`Item/FastFood.cpp`)

- Opens 10:00 (`:56`). At open, 10 customers are created (`:61`) with tower arrival U(10:00, 20:00) on the absolute clock (`:65`). They spawn at the main lobby and walk the lobby route.
- Stay: 20 real seconds at 1x (`20*kBaseSpeed` days, about 0.104 day) (`:94`).
- Closes 21:00. Population = number of customers who arrived. Income = `population*200 - 2000` (`:72-79`).
- With 10 customers max, net income is <= $0. This is a placeholder.
- Customer types: Man, Woman1, Woman2, WomanWithChild1 (`:140-141`). Sprite shows busyness as `ceil(people/5)` capped at 2 (`:49`).

**Restaurant** (`Item/Restaurant.cpp`)

- Open 17:00-23:00 (`:56, 72`). 10 customers arriving U(17,22) (`:61-65`).
- Stay 20 s at 1x (`:94`). Income `population*400 - 4000` (`:78`). Max net is again $0.

**Party Hall** (`Item/PartyHall.cpp`)

- Open 13:00-17:00, then a flat +$10,000 every day including weekends (`:51-63`). No visitors ("TODO: Specify party hall income", "TODO: Make people arrive").

**Movie Theatre** (`Item/Cinema.cpp`)

- Two shows: doors open 13:00 and 19:00, the movie plays 15:00 and 21:00, closing 17:00 and 23:00 (`:76, 94, 101`).
- At each open, 50 customers are created and sent immediately (`:83-90`). At close, income = `customers*500 - 2000` = $23,000 per show, since customers.size() is always 50 (`:107`). That is about $46,000/day. Placeholder.
- 15 movie variants (`:19`). No weekday dependence.

**Metro** (`Item/Metro.cpp`)

- Open 07:00-23:00, visual only (`:67-76`). No trains, no customers (TODO.md "Item::Metro").
- Only underground (y > 0 blocked, y = 0 blocked by the ground rule), only one. Quirk: y = -1 with height 3 would overlap floors 0-1.
- For routing, the middle floor is enterable when the destination is one floor below the entrance (`Game.cpp:1025-1029`).

**Lobby / Floor**: no simulation.

### 2.2 Hotel (SDL only; OS has none)

- **Room states**: occupied/asleep/vacant-clean/dirty/infested (`hotel/hotel.cpp:18-20`). Bitmap slice index (`:273-297`):
  - 0 occupied
  - 2 asleep
  - 3 vacant
  - 5 dirty
  - 7 infested
  - Each has a +1 "unlit" variant, except asleep.
  - Infested is never set by the logic (cockroach state exists but is not driven).
- **Guests**: single 1, double 2, suite 2 (`singleroom.cpp:40-43`, `hotel.cpp:32-35`). Types: Man then WomanB (`:64-69`).
- **Check-in**: `occupyAt = random(17:00, 21:00)` today (`hotel.cpp:249-252`). Requires a lobby route (`occupiable.cpp:86-116` -> `isSufficientlyAttractive()` = reachable from lobby). An unattractive occupied room vacates at a daily 5:00 check.
- **Guest day** (`people/individuals/hotelguest.cpp:94-174`):
  1. Go to room, pause 0.5 h.
  2. Dinner: if 18:30-21:00, leave the tower ("destination NULL" = outside; restaurants are not actually searched), pause 0.75 h.
  3. Return to room.
  4. Sleep at random(23:00, 25:30).
  5. Wake random(6:00, 8:00) next day.
  6. Check out at random(now+0.25 h, 11:00) and leave. The hard end is 12:00.
- **On checkout of all guests**: room becomes vacant and **dirty**, and a `kHotelVacated` event is sent (`hotel.cpp:57-80, 127-144`).
- **Housekeeping**: 6 housekeepers per housekeeping facility (`housekeeping.cpp:31-34`).
  - On vacate, the first idle housekeeper is assigned (`:123-136`).
  - Cleaning = arrive, pause 0.25 h, set dirty=false on arrival (`hotel.cpp:118-125`, `janitor.cpp:123-143`).
  - Next job: dirty, unassigned room with smallest vertical distance, then leftmost (`housekeeping.cpp:46-85`). Otherwise return to base and idle (pause 24 h).
- **Hotel income / rent per night: not implemented.**
- OS TODO.md "Hotels" summarizes the intended original behavior:
  - populated after 17:00
  - dinner about 0.5 h after entering
  - asleep 23:00-1:30
  - up 6-8, leave 8-10, leaving rooms dirty
  - housekeepers clean for a fixed absolute time

### 2.3 Office in SDL (alternate numbers)

- Move-in window 7-13 on weekdays (`office/office.cpp:97`). `occupyAt = random(7, max(7, now+0.5))` (`:128`).
- +$10,000 on occupy and -$10,000 on vacate (`:48`).
- Workers (`officeworker.cpp:80, 100`): arrive random(7,10), stay 0.5 h minimum, depart random(17,20). Weekends skipped (`:75-77`).
- Fast food open 10-21, restaurant 17-23 (`fastfood.cpp:44`, `restaurant.cpp:44`). Generic facility lights on 7-17 (`facility.cpp:117-120`).

---

## 3. Time

### OS clock (`Time.cpp`)

- `kDayDuration = 3.2` real minutes per game day at speed 1 (`:7`). `kBaseSpeed = 1/60/3.2` days per real second (`:8`).
- Speed modes: 0 = pause, 1 = 1x, 2 = 2x, 3 = 4x (`Game.cpp:921-936`). Ramp-up is smoothed (`(speed*dt + anim*0.1)/(dt+0.1)`); slow-down is instant (`Time.cpp:64-74`).
- Day = 2600 "frames" (from TDT). Hour mapping, with absolute day fraction a: stage = a*2600 (`Time.cpp:11-38`).

| Game hours | Frames | Real s at 1x | Game min / real s (1x) |
|---|---|---|---|
| 0:00-1:00 | 0-100 | 7.38 | 8.1 |
| 1:00-7:00 | 100-300 | 14.77 | 24.4 |
| 7:00-12:00 | 300-700 | 29.54 | 10.2 |
| 12:00-13:00 | 700-1500 | 59.08 | 1.0 (lunch rush slow-mo) |
| 13:00-24:00 | 1500-2600 | 81.23 | 8.1 |

Multiply the rates by 2 or 4 for speeds 2 and 3. A full day is 192 s at 1x.

- Calendar (`Time.cpp:58-60`):
  - `day = floor(a) % 3`: 0 = "1st WD", 1 = "2nd WD", 2 = "WE" (`TimeWindow.cpp:55-57`)
  - `quarter = floor(a/3) % 4 + 1`
  - `year = floor(a/12) + 1`
  - So a quarter is 3 days and a year is 12 days.
- Start: `time.set(7/78)` gives 5:00 on day 0, Q1, Year 1 (`Game.cpp:30`).
- Day boundary is midnight in OS. Original: the date flips at frame 2300 = 0:00 (TDT:84-87).
- **Rent day**: Office/Condo pay on day 0 at 5:00, which is once per quarter (`Office.cpp:104`, `Condo.cpp:108`). Entertainment pays daily at closing.
- Daily events: 3:00 condo jitter, 5:00 weather roll + office reschedule + rooster, 6:00 birds, 9:00 bells, 18:00 evening birds (`Game.cpp:506-509`, `Sky.cpp:30-33`).
- Weather: each day at 5:00 there is a 1/3 chance of rain (`Sky.cpp:31`). Rain is visible 8-16 with fades 7-8 and 16-17.
- Sky: night <5 or >=19, dawn 5-7, dusk 17-19 (`Sky.cpp:42-61`).

### Original SimTower time (TDT:72-102)

- Frame 0 = 7:00, 2600 frames/day.
- Periods:
  - 7-12 = 400 frames (45 s/frame)
  - 12-12:30 = 400 frames (4.5 s/frame)
  - 12:30-13 = 400 frames (4.5 s/frame)
  - 13-17, 17-21, 21-1 = 400 frames each (36 s/frame)
  - 1-7 = 200 frames ("126 s/frame"; the arithmetic actually gives 108)
- These periods "refer to the number of the block in the elevator panel" (elevator scheduling time blocks). Period 7 is merged into period 6.
- Day counter: int32, Day 0 = WD1/Q1/Y1, Day 1 = WD2, Day 2 = WE, Day 3 = WD1/Q2. It rolls over after year 1000.

### SDL clock (`tower/time.cpp`)

- Time in hours, start 5:00 (`:19`).
- Assumes 15 Hz, so hours per real second = (s/frame)*15/3600 (`:186-220`):
  - 7-12: 0.1875 h/s
  - 12-13: 0.01875 h/s
  - 13-1: 0.15 h/s
  - 1-7: 0.525 h/s
  - That gives about 171 s/day.
- The "logical day" starts at 1:30 so overnight events belong to the previous day (`:74-77, 130-133`).

---

## 4. Economy

| Item | Value | Source |
|---|---|---|
| Starting funds (OS) | $4,000,000 | `Game.cpp:25` |
| Starting funds (SDL) | 20e6 (comment says 2,000,000) | SDL `tower/funds.cpp:19-21` |
| Original money scaling | UI shows 100x the internal value, except the Finance window | TDT:64-69 |
| Construction | item price debited on placement | `Game.cpp:371, 377` |
| Floor | $500 per new cell | `Game.cpp:800, 811` |
| Lobby extension | `price*4/diff` (looks inverted; likely meant `price*diff/4` = $5,000/cell, matching SDL's $5,000/cell) | `Game.cpp:360` |
| Extra elevator car | $80,000 | `Game.cpp:157` |
| Office rent | $10,000/quarter + $10,000 refundable deposit | `Office.cpp:27, 98, 111, 116` |
| Condo | $5,000/quarter + deposit (placeholder) | `Condo.cpp:20, 101, 116, 120` |
| Fast food | `200*customers - 2000`/day | `FastFood.cpp:78` |
| Restaurant | `400*customers - 4000`/day | `Restaurant.cpp:78` |
| Party hall | +$10,000/day | `PartyHall.cpp:62` |
| Cinema | `500*50 - 2000` = $23,000 per show, 2 shows/day | `Cinema.cpp:107` |
| Maintenance / expenses | **not implemented** | |
| Bulldoze refund | none | `Game.cpp:393-405` |

Original finance window categories (TDT:311-330):

- Income types (10): Office, Single Room, Twin Room, Hotel Suite, Shops, Fast Food, Restaurant, Party Hall, Theater, Condo.
- Maintenance types (10): Lobby, Elevator, Exp. Elevator, Ser. Elevator, Escalator, Parking Ramp, Recycling Center, Metro Station, Housekeeping, Security.
- Header fields: `currentBalance`, `otherIncome`, `constructionCosts`, `lastQuarterMoney` (TDT:37-42). Each tenant has a `rentClass` byte (TDT:139), which suggests a per-tenant rent tier (the original's rent-level setting).

Population (OS) = sum of `item->population` (`Game.cpp:496-503`):

- office 6 when occupied
- condo = occupants
- fast food/restaurant = customers counted at closing (persists until the next close)
- cinema/party hall contribute 0

---

## 5. People and transport

**Person** (`Person.h:13-23`, `Person.cpp`):

- Types: Man, Salesman, Woman1, Child, Woman2, Housekeeper, WomanWithChild1, WomanWithChild2, Security.
- Width 8 px, or 16 px for types >= Housekeeper (`Person.cpp:63-66`).
- `stress` is in [0,1].
- A journey is a queue of `Route::Node{item,toFloor}`. `next()` removes the person from the current item and adds them to the next item.

**Route finding** (A*, `PathFinder/`):

- Graph: one transport node per stair/escalator/elevator stop, linked left/right along a floor. Each floor also has a "floor node" (`GameMap.cpp`).
- Routes go from the main lobby to an item's center x at `position.y + entrance_offset` (`Game.cpp:1020-1034`, `Item.cpp:127-134`).
- An item with no route shows the "noroute" icon (`Item.cpp:74-82`) and is unattractive. It never gets tenants and vacates at the next rent check.
- Costs (`PathFinder/MapSearchNode.h:38-45`, applied in `MapSearchNode.cpp:20-72`):
  - walking 1 per cell
  - floor 5 per floor
  - escalator 10 (+5)
  - stairs 30 (+5)
  - standard elevator boarding 170 (= 30*3 + 80) + 5 per floor
  - express/service boarding 20 + 5 per floor
  - walking on floor 0 is free
  - final walk > 80 cells adds an inhibitory `80 + dist/80*10000`
- Transfer limits (`MapSearchNode.cpp:236-269`):
  - at most 2 elevators
  - a 2nd elevator is allowed only when transferring on a floor with `y % 15 == 0` (sky lobby) and if no stairs/escalators were used
  - stairs: max 3 (`numStairs <= 2 - numEscalators` before boarding)
  - escalators: max 6
  - mixed: at most 1 stair + 2 escalators, or 2 stairs + 1 escalator
  - service elevators are usable only for `serviceRoute=true` (nobody uses that yet), and normal elevators only for non-service routes (`:259-260`, `GameMap.cpp:89`)
  - TODO.md notes the sky-lobby existence is not checked
- SDL limits: rejects at >= 2 elevators, >= 4 stairs, >= 6 escalators. An elevator transfer happens only on `floor % 15 == 0`. The SDL route also has a NoServiceElevators/OnlyServiceElevators option (SDL `tower/route.cpp:223, 237-249`).

**Stairs/escalator**: fixed 1.5 real s per traversal at 1x (`Stairlike.cpp:8, 31`). Stairs and escalators move the same speed; the only difference is pathfinding cost. They connect only their bottom and top floor (`Stairlike.cpp:60-63`).

**Elevators** (`Item/Elevator/`):

- Defaults: accel 7.5 floors/s^2, vmax 10 floors/s, capacity 21 (`Elevator.cpp:20-22`). Service uses the same defaults.
- Express: accel 20, vmax 30, capacity 21 (`Express.h:23-24`). SDL has identical numbers (`elevators/elevator.h:69-71`, `express.h:19-20`).
- Units are floors per real second at 1x; `journeyTime += dta/kBaseSpeed`, so elevators scale with game speed.
- Motion: trapezoidal. `v = min(vmax, sqrt(2*q*a*s))` with q = 1/3, `t0 = v/2a`, `t1 = s/v + 2*t0` (`Car.cpp:151-213`).
- Door open/close 0.1 s, dwell before closing 0.15 s, 0.05 s per person boarding/alighting (`Car.cpp:9-12`). SDL uses 0.1 / 0.025 (SDL `car.cpp:342, 356-368`).
- Car state machine (`Car.h:43-49`, `Car.cpp:246-308`): Idle -> Moving -> OpeningDoors -> Hauling (unload first, then load the same-direction queue) -> ClosingDoors -> pick destination. Empty after hauling -> Idle.
- Shaft height: clamped to 1..31 floors (`30+1`) when dragging motors (`Elevator.cpp:181-182`). A new shaft is 1 floor tall with 1 free car (`Elevator.cpp:41`, `Standard.h:14`).
- Express stops only at `floor % 15 == 0` (lobby floors) and at **all** basement floors. Bug: it returns true for any floor < 0, even outside the shaft (`Express.h:27-32`). SDL: active and `% 15 == 0` (SDL `express.cpp:41`).
- Stops can be toggled per floor (`unservicedFloors`) by clicking the shaft with the "finger" tool (`Game.cpp:427-434`).
- Car count: **no cap** in code. Each extra car costs $80,000 and is placed on the clicked floor (`Game.cpp:146-161`, `Elevator.cpp:222-229`).
- Queues: one per (floor, direction), drawn up to 400 px wide (`Elevator.cpp:256-269`). The first person in a queue calls the elevator.
- Dispatch (`Elevator.cpp:292-377`):
  - `respondToCalls` picks the uncalled-answered queue with the longest wait.
  - It sends the nearest **idle** car.
  - A moving car with passengers goes to the nearer of (a) the nearest passenger destination and (b) the nearest called same-direction queue ahead of it (distance >= 0.5 floor), if it is not full.

**Stress** (`Item/Elevator/Queue.cpp`):

- +1/30 per real second at 1x while waiting in an elevator queue (`kSecondsUntilStressed = 30`, `:13, 74`).
- At stress >= 1 the person is forcibly advanced to the next journey node (`:75`). In effect they teleport past the elevator. Nothing else adds stress (stairs/walking add none).
- Display color in the queue: black, pink (255,128,128) if > 0.4, red if > 0.8 (`:124-126`). The original has 5 people palettes: normal black, stressed red, named blue, VIP yellow, plus one more (`SimTowerLoader.cpp:477-490`; SDL `simtower-names.cpp` "people/stressed", "people/named", "people/vip").
- Office arrival halves stress (`Office.cpp:199`). It resets to 0 at the 5:00 reschedule (`Office.cpp:244`).
- **Stress does not feed any evaluation or move-out.** Move-out depends only on route existence.

Original per-person record (TDT:217-245): `stress` WORD and `eval` WORD per person. Per-tenant person counts: Condo 3, Office 6, Fast Food 48.

---

## 6. Events

| Event | Status | Evidence |
|---|---|---|
| Fire | not implemented; bitmaps only (flames x4, small flame, chopper, burned floor; tenant type 48 "Burned Area") | `SimTowerLoader.cpp:459, 641-643, 657`; TDT:190 |
| Terrorist bomb | not implemented; alert bitmap 0xA710 | `SimTowerLoader.cpp:653` |
| VIP visit | not implemented; alert bitmap + yellow VIP people palette | `SimTowerLoader.cpp:655` |
| Treasure | not implemented; alert bitmap | `SimTowerLoader.cpp:656` |
| Santa | not implemented; sprite `deco/santa` + sound | `SimTowerLoader.cpp:647, 1078` |
| Cathedral wedding / Tower | not implemented; SDL lists bitmaps for wedding, bride and groom, "cathedral with tower rating" | SDL `simtower-names.cpp` (0xDE8-0xCE9, 0x828) |
| Rain | implemented, cosmetic (1/3 days) | `Sky.cpp:29-33` |
| Star up | sound `rating/increased`, alert 0xA716 | `Game.cpp:889`, `SimTowerLoader.cpp:658` |

---

## 7. Rating and evaluation

- OS `rating` is an index: 0 = 1 star ... 4 = 5 stars, 5 = TOWER. The time-window strip has row i = i+1 filled stars and row 5 = the Tower emblem (`SimTowerLoader.cpp:545-553`, `TimeWindow.cpp:33-37, 72-79`). The new game starts at 0 (`Game.cpp:26`).
- Only rule implemented: index 0 -> 1 (2 stars) at population >= 300. At index 1 with population >= 1000 it only shows the message "Your tower needs security." The security check is a TODO (`Game.cpp:906-919`). Nothing above that. F3 debug-sets the rating to 1 (`Game.cpp:105`).
- SDL: rating starts at 1 (stars) and only changes via debug keys 1-6 (SDL `tower/environment.cpp:21`, `game.cpp:349-355`).
- Original save: `WORD level`, where 1-5 = stars and 6 = TOWER (TDT:35-36).
- **Not encoded anywhere**: the 1000/5000/10000/15000 thresholds, facility prerequisites (security, medical, recycling, metro, hotel suites, VIP, cathedral), and the blue/yellow/red tenant evaluation.
- The only evaluation-like signal is the elevator-queue stress tint.
- Win condition: none.

---

## 8. What OpenSkyscraper does NOT implement (gaps to fill elsewhere)

- **Hotels**: single/twin/suite, housekeeping, nightly income, dirty/cockroach rooms. Only partially present in SDL; SDL has no income.
- **Missing facilities**: shops (SDL has the descriptor only), parking ramp/space, recycling center, security, medical center, cathedral, condo-as-sale (a $ sale price instead of rent), SECOM (TDT notes ID 17).
- **Evaluation system**: per-tenant blue/yellow/red eval, eval-driven move-outs, the eval map overlay.
  - The bitmaps "ui/map/key/eval", "pricing" and "hotel" exist (SDL `simtower-names.cpp`).
- **Rent-level setting**: the per-tenant `rentClass` exists in TDT but is unused.
- **Star thresholds and prerequisites** above 2 stars, and the TOWER win condition.
- **Expenses**: quarterly maintenance (see the 10 categories in TDT), the finance window, and the quarter-end report.
- **Lunch behavior**: office lunch trips to fast food, customers choosing shops/restaurants by proximity, and per-item real patronage/capacity.
- **Real income formulas**: entertainment numbers are placeholders netting <= $0, except cinema and party hall.
- **Metro**: trains and underground customers.
- **Parking-driven commuting**: workers arriving by car.
- **Events**: fire, bomb, VIP, treasure, Santa, plus the weekend/holiday variations.
- **Elevator rules**: no car-count cap; the original's 8 cars/shaft is not encoded anywhere in the repo. No elevator scheduling by time block (the TDT period table hints the original had per-period scheduling). No standard-elevator waiting mode or express capacity differences.
- **Stairs**: no stress or walk-time penalties for stairs; no "too far to walk" tolerance other than the pathfinding cost.
- **Horizontal geometry**: no tower width limit; no "Metro must be lowest" rule (TODO.md).
- **Sky lobbies**: transfer points are not verified to have a lobby built.
- **Extra time rules**: no holiday/week structure beyond WD/WD/WE.

---

## 9. Original-SimTower reverse-engineering material in the repo

**`doc/simtower/TDT_format.txt`** (save format, version WORD 0x2400):

- **Header**: version, level (1-6), 4 money DWORDs, `frameTime` WORD, `currentDay` int32, lobby height?, 8 unknown bytes, scroll position, 518 unknown bytes.
- **Floor map**: 120 entries x {numTenants, left/right edge, tenants[18 bytes]: left, right, type int8 (negative = under construction), status, perTypeDataIndex, peopleOffset, indexInFloor, rentClass}, followed by `indexMap[94]`.
- **Tenant IDs** ([*] = unconfirmed by the author):
  - 0 Floor
  - 3 Single*, 4 Twin*, 5 Suite*
  - 6 Restaurant*
  - 7 Office
  - 9 Condo
  - 10 Retail*
  - 11 Parking Space*
  - 12 Fast Food
  - 13 Medical*, 14 Security*, 15 Housekeeping*
  - 17 SECOM
  - 18 Movie
  - 20 Recycling*
  - 24 Lobby
  - 29 Party Hall*
  - 31 Metro
  - 34 Movie Theater
  - 36 Cathedral
  - 42 "structures"
  - 45 Parking Ramp*
  - 48 Burned Area
  - Gaps (19, 21, 30, 32-33, 35, 37-40) are probably the upper floors of multi-floor items.
- **Extents**: in floor units of about 8 px, half-open. Transports are not tenants and live in separate blocks.
- **People**: `numPeople` x {tenantFloor, tenantInFloor, numInTenant, tenantType, status, currentFloor, 5 unknown, stress WORD, eval WORD}.
- **Retail block**: 512 x 18-byte entries.
  - Restaurant names: English Pub, French, Chinese, Sushi Bar, Steak House.
  - Fast food names: Japanese Soba, Chinese Cafe, Hamburger Stand, Ice Cream, Coffee Shop.
  - Shop names: Men's Clothing, Pet Store, Flower Shop, Book Store, Drug Store, Boutique, Electronics, Bank, Hair Salon, Post Office, Sports Gear.
- **Elevators**: 24 variably-sized records. The original therefore allows **24 elevator shafts**.
- **Population/income/maintenance arrays**: 10 + total each.
- **Stairs/escalators**: 64 x 10-byte records {present, type 0 = escalator / 1 = stairs, x, baseFloor, 4 unknown}. That allows **64 stairlikes**.
- **Named tenants**: 16-byte names at the end of the file.

**`doc/simtower/Used Resource Types.md`** (NE resource types):

- 0x8002 bitmaps
- 0x8005 dialogs (message texts)
- 0xFF02 per-cell bitmaps (lobbies)
- 0xFF03 palettes (day/night)
- 0xFF0A sounds

**Resource ID maps** (useful for extracting original art and sounds from SIMTOWER.EXE):

- OS `source/SimTowerLoader.cpp:342-668` (bitmaps) and `:1042-1100` (sounds), with IDs as 0x8000 + n.
- SDL `source/game/simtower-names.cpp` (the same IDs without the 0x8000 bit, with descriptive names). Selected entries:
  - office 0x5A8-0x5AB
  - single 0x4A8-0x4AB, double 0x4E8-0x4EF, suite 0x528-0x52B
  - condo 0x628-0x636
  - fast food 0x6E8-0x6F1
  - restaurant 0x568-0x571
  - shops 0x668-0x674
  - cinema 0x868/0x8A8/0xC68-0xCA9
  - party hall 0xB28/0xB68
  - parking 0xEE8-0xEEA/0x6A8-0x6A9
  - recycling 0x8E8-0x92E
  - metro 0xBA8-0xC29, 0xF28
  - cathedral 0xCE8-0xDEA, 0x828
  - security 0x768
  - medical 0x728-0x72A
  - housekeeping 0x7A8
  - elevators 0x428-0x42B
  - stairs 0x968-0x9A9, escalator 0xAA8/0xAE8
  - lobbies 0x9E8-0x9EA, 0xA28+, 0xA68+, 0xFE9/0xFEA (double/triple lobby)
  - people 0x2BC-0x2BF, 0x468-0x469
  - fire 0xF68-0xFA8
  - alerts 0x2710-0x2716
  - sky 0x351-0x35B
  - Sounds: construction 0x1B58-0x1B5B, ambience 0x1388-0x138D/0x271B-0x271C, rating up 0x2710, tower 0x2718, Santa 0x2712, cash 0x271D, movies 0xA329+.
- Palette animation: security offices and others animate by cycling palette indices 197/198-200 (`SimTowerLoader.cpp:461`, `:1012-1033`). The sky uses palette swaps 0x83E9-0x83EB for twilight/night/overcast (`:894-958`).
- The original's "double lobby" (0xFE9) and "triple lobby" (0xFEA) bitmaps imply 2- and 3-floor-tall lobbies exist. OS does not support them.

**`doc/simtower/Windows NE Format.txt`**: generic NE executable format reference, not game mechanics.
