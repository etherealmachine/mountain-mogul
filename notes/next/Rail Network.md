---
title: Rail Network
kind: plan
status: done
---

# Rail Network

Railways as a network the player lays and edits, instead of one line from the map's edge to a station: track laid and reshaped like creeks and roads, joined into junctions, entering from the map's edge at entry nodes; stations as buildings placed on the track; each stretch of track steam, electric or high-speed, and trains routed over the network. Follows [[Trains and Tunnels]]; touches [[Transit]], [[Parking and Roads]], [[Demand]], [[Water Editing]].

## From the user (2026-10-14)

- Track is its own thing, placed like rivers and roads: nodes with a midpoint handle to grab for a new node, ends that snap together; and an entry node for rail like the road's.
- A separate station building; the track itself is set to steam, electric or high-speed.
- Trains: if the rail connects two entry nodes it's a through service; with just one, out and back. Multiple stations.
- Styles: a speed limit per stretch. On a network with electric or high-speed track, steam (unelectrified) stretches show a warning; where high-speed track exists, electric stretches show a slow sign. Clicking a stretch upgrades it, for a price, and clears the sign.

## Decided

- **Track** (`RailTrack`): a polyline of points with a style, smoothed into a curve, laid out on its grade like a road (`world/grade.go`: cut, filled, tunnelled), its profile saved. Each end is loose or at a **joint** (`RailJoint`): where tracks meet, or an **entry** at the map's edge. Ends pin to their joint's height, so tracks meet level.
- **Laying track**: the Rail tool's three styles lay points like the Footpath tool (click, click, click the last again or Enter to finish); an end within 25 m of the map's edge snaps to it as an entry; an end on another track's end joins it, and on its middle splits it there into a junction (as roads split). Too steep (a style's grade between joints, or a point pair) is refused.
- **Editing**: with no tool, a track's points show as handles and a smaller handle at each midpoint, as creeks do in the editor: drag a point, grab a midpoint to add one, right-click a point to delete it; dragged ends snap and join as when laying. The Remove tool removes a track (and a station on it); a joint with no tracks left goes.
- **A stretch** is one track between its joints. Clicking one opens its popup: style, length, speed, and an upgrade to the next style for the difference in price a metre.
- **Stations** (`BuildingStation`): placed on a track by the station tool, snapping to it, needing a platform's length of track clear of joints; the platform beside the track on one side, as long as the network's train; the house beside it. Its popup has the interval and the trains through it.
- **Routes**: a station's network (the tracks it's connected to) is served from its entries. With two or more, through services: each train runs entry to entry the shortest way through the station (alternating which way), stopping at every station it passes. With one, out and back from it, the same. With none, no trains (the popup says why).
- **The train** on a route is the best style anywhere on the network (any high-speed track means a high-speed set, else any electric an electric unit, else steam), and runs at each stretch's speed: slow over electric stretches if it's high-speed. An electric or high-speed train can't run over steam track: a route through steam track is blocked, those stretches show a warning, and the stations behind them a "no service" line, until they're upgraded.
- **Guests** book on a station, weighted by its train's pull as before (`railShares`); a train lets off the guests booked for each station it stops at; leaving guests walk back to their station.
- **No old railways**: saves with the old one-line railways drop them (pre-release, breaking is fine).

## Built

All three steps together (they share the routing and the stations).

- **Model** (`world/rail_track.go`): `RailTrack` (style, points, the joints at its ends, its laid-out samples and profile) and `RailJoint` (where, the rail's height, entry or not). A track's ends sit at its joints' heights and it holds its style's grade between (`layTrack`, the shared grade code), graded into the ground (`GradeTrack`: cut, filled, tunnelled, plowed, buildings' ground left be). Splitting a track for a junction keeps each half's profile and tunnels (`carryProfile`); joints merge (`MergeJoints`) and move (`MoveJoint`, an entry once it's within 25 m of the map's edge, snapped onto it); removing a track removes its stations and any joint left bare. `RailLine` is now just a laid-out line (a track's, a funicular's).
- **The Rail tool** (one button a style, `scene/rail_tool.go`): click points; it starts and finishes on a joint (within 8 m), on a track (within 6 m: a junction there), or anywhere (an entry at the map's edge); clicking the last point again finishes it loose (a buffer stop). The ghost is the laid-out track, red when it's too short (20 m) or too steep (its style's grade between its ends' heights), or can't be paid for; the cost preview counts tunnels. Joints show as dots while laying (entries blue).
- **Editing**: with no tool, clicking a track selects it and opens its popup; its points show as orange handles and dots between them, as creeks do in the editor. Drag a point; drag an end to move its joint (and every track at it), dropping it on another joint to join them or on a track to make a junction; grab a dot to add a point; right-click a point to take it out. An edit that makes track too steep, or whose extra cost can't be paid, is put back (`SnapshotRail`/`RestoreRail`); else the difference is charged and the ground graded. The Remove tool removes a track or a station (no refund, like the rest).
- **Track popup**: length, speed, the sign's reason, and an upgrade to the next style for the difference a metre of open track (refused if the new style's grade doesn't hold).
- **Stations** (`world/rail_trains.go`, the Station tool): beside the track nearest the mouse (within 15 m), on the mouse's side, needing the network's train's platform length clear of the track's ends and other stations, and not in a tunnel; priced by the network's train (steam $150k, electric $300k, high-speed $600k). The platform (`stationsMesh`) is as long as the network's train and grows when it's upgraded; the house (`rail_station.py`) stands behind it. A station keeps its place by its platform's middle, re-found on the nearest track whenever track changes (`PlaceStations`). Its popup: its trains (style, through or out and back, riders; or why none), the interval (15 to 240 minutes), booked, and here by train.
- **Networks and routes**: a network's train is its best track's style. Each station's train every interval: with two or more entries reachable, through from one to another the shortest way (Dijkstra over track the train can run on), the other way next time; with one, out and back, running half a train past the platform to turn. It stops at every station it passes (once each), runs at each stretch's speed, and brakes for its stops. Electric and high-speed trains don't run on steam track: routes go round it, or the station has no service.
- **Signs**: a yellow warning triangle over steam track in a network whose train is electric or high-speed, and a speed-limit roundel (its km/h) over electric track in a high-speed network.
- **Guests** book on a served station, by its train's pull (12, 30, 45%, at most 80% together; all without parking), and get off where they booked; leaving, they walk back to their station.
- **Saves**: tracks (with their profiles), joints and stations; old saves' railways and their station buildings are dropped.
- **Checked** (headless and on screen, Goals Test): electric track from the west edge to a junction, steam track on to the south edge, electric track to a dead end; a station near the junction got an out-and-back electric service from the west (the steam stretch blocking the south, its warning up) and 278 of about 960 guests by 13:00. Upgrading the steam stretch to high-speed turned it into a through service west to south with slow signs over the electric track. Laying and editing by mouse are untested in play.

- **Styles drawn** (`railLooks`, `render/rail_mesh.go`): steam track on brown ballast with wooden sleepers and rusty rails; electric on grey ballast with concrete sleepers and masts with arms; high-speed slab track (concrete slab and twin blocks on a gravel shoulder) with cable troughs, gantries over both sides and a messenger wire with droppers. Platforms by the network's train: gravel and white edge for steam, concrete with a short green canopy for electric, pale paving with a long canopy for high-speed. Only tracks laid out afresh or restyled get new meshes, so a drag no longer rebuilds them all.
- **In the editor** (`scene/editor_rail.go`): the Transport menu has the three Rail buttons and Station; the tools and editing are the game's (`railEnv`, shared), free; a track's popup has a button for each other style instead of the upgrade; a station's popup sets its interval; Remove and Delete take track out.
- **Finishing at the edge**: with track being laid, the mouse off the map points the ghost at the map's edge where the line from the last point toward the mouse leaves it; a click there, or anywhere within 25 m of the edge, finishes the track on an entry (an entry is always an end).
- **The course over the ground**: while laying track (and roads and paths), the line as it'll be laid is drawn over everything (`drawCourse`), along the ground or the line where it's higher, dashed through tunnels, red where it can't be built: the mesh preview sinks into the cuts it would make.
- **Tunnel mouths and cuts** (`GradeBed`, every graded line): cuts bank at a rock cut's slope (rail 0.6, roads 0.75, paths 1 run over rise; fills stay 1.5), so a cut into a sidehill meets the hill; behind each mouth past the portal's dark inside, as wide as its portal, the hill isn't cut below the portal's headwall, so it comes down onto the portal instead of being dug out behind it; across the opening and in front of the face between its wing walls the ground stays at the bed, so the hill shows neither through the opening nor over the face; and trees go wherever the ground was cut or filled.
- **A station's ground** (`world/rail_station_grade.go`): a level pad on its side of the track at the track's bed (where the platform and house stand), along the platform (at least the house's length) and out 22 m past the house, banked out like the track's bed, its trees cleared; graded when it's placed or dropped after a drag, and whenever the railway changes (a grown platform). Station buildings no longer count as claimed ground for other grading.
- **Moving a station**: dragging a station (structure edit, game and editor) slides it to the spot on the track nearest the cursor where a station can stand, platform and house together; R turns its platform to the other side of the track.

## Steps

1. Done: **Track and joints**: the model (tracks, joints, entries), laying out and grading per track with pinned ends, save and load; the Rail tool laying track with snapping, joining and splitting; editing points and midpoints; removal; drawing (as railways now, entry posts); the stretch popup with upgrades. The old `RailLine`, its tool and its trains go (trains come back in step 3).
2. Done: **Stations**: placing on track, the platform and house drawn from the network's train, removal, the popup.
3. Done: **Trains on the network**: routes (through or out and back), speeds by stretch, blocked routes, the warning and slow signs over stretches, booking per station, stops at every station on a route, drawing over a route's tracks and joints.

## Log

- 2026-10-14: Planned from the user's notes and two answers (through services, speed limits by stretch with warning and slow signs).
- 2026-10-14: Steps 1–3 built together; mouse laying and editing still to be tried in play.
- 2026-10-14: Track and platforms drawn in their style; railways in the editor; stations move along their track. Mouse use in the editor untried too.
- 2026-10-14: Clicking off the map (or at its edge) finishes track there, on an entry.
- 2026-10-14: Station placement put the platform on the side away from the mouse (and a dragged station flipped): fixed. Laying track, roads and paths draws the course over the ground.
- 2026-10-14: Tunnel mouths: the hill was dug down to the rail behind the portal (the bank radiated round the last open sample); now it meets the headwall's top, and cuts are steeper. Tall fills ending in cliffs noted in [[Bugs]].
- 2026-10-14: The raised hill sloped from the face across the grid, into the opening and over the track; now it rises only behind the portal's dark inside, and the ground stays at the bed across the opening and between the wing walls.
- 2026-10-14: Stations level their ground.
- 2026-10-14: Tunnel mouths as special places (the user's idea): the cut stops at the face and the hill behind is trimmed back at the cut's slope from the portal's top, not dug into a bowl; the terrain isn't drawn where the ground's grid slopes up across the face (terrain.frag `inMouth`, the mouths as uniforms: `render/tunnel_mouths.go`); roads and paths take the bed's height there and stop at the face; the portal model has stone over the opening back past its dark inside.
- 2026-10-14: The ghost went red for a point on the way (judged as if the track ended there: too short, too steep end to end) though the click only adds a point; now it's red only when the click would finish the track and that can't be built.
