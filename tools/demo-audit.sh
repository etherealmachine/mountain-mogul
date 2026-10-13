#!/bin/sh
# Fails if the demo binary names anything the demo leaves out: function,
# type and field names, and the strings the left-out code would print.
# Usage: tools/demo-audit.sh build/demo/mountain-mogul-demo
set -u
bin=${1:?usage: demo-audit.sh BINARY}

# Case-sensitive patterns, one per line. Kept specific, so a name the
# demo keeps (a toolbar, guests' rented skis) doesn't trip them.
patterns='
Heli
heli_
helicopter
helipad
Gondola
gondola
SnowGun
snow_gun
Snow Gun
sg_on
HS6Pack
6Pack
6-Pack
6pack
ServiceBar
BuildingBar
ShellTent
tent_
ServiceRentals
Rental Shop
BuyParcel
toolLandBuy
purchasable parcel
Editor
editor
Testbed
testbed
/debug/pprof
net/http.
QueryServer
queryserver
internal/geo
OpenStreetMap
runScreenshot
-screenshot
Snow guns
Too warm
Heli fares
Fare per ride
High-Speed 6-Pack
HS 6-Pack
Rental shop
Rental price
Drink price
Available for purchase
Buy for $
Scenario Editor
Testbeds
Save as Scenario
New blank scenario
Restarted with seed
GIF recording
Logging skier
'

found=0
dump=$(mktemp)
strings -a -n 4 "$bin" > "$dump"
echo "$patterns" | while IFS= read -r p; do
	[ -z "$p" ] && continue
	n=$(grep -cF -- "$p" "$dump")
	if [ "$n" -gt 0 ]; then
		echo "== $p ($n)"
		grep -F -- "$p" "$dump" | head -5 | cut -c1-160
	fi
done | tee "$dump.hits"
if [ -s "$dump.hits" ]; then found=1; fi
rm -f "$dump" "$dump.hits"
if [ "$found" -ne 0 ]; then
	echo "demo-audit: the demo binary names left-out features (above)" >&2
	exit 1
fi
echo "demo-audit: clean"
