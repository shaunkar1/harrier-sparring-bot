# Harrier: independent Mech Royale FFA sparring bot

Standalone **Go 1.23** bot written from the official `protocol.md` and `mechfight-kit.tar.gz` specification. Harrier does **not** import code from `shaunkar1/proj1`, is not a fork of that bot, and is **not submitted** to the Builders leaderboard.

Its goal is to act as a meaningfully different opponent in your local 16-way free-for-all simulator. It speaks `mechfight/1` via JSON Lines on **stdin/stdout** with a static, dependency-free Linux/amd64 Docker image. Local native binaries work with your simulator's existing `ext:` hook.

## Behaviors

- **Harrier (default):** Avoid fair head-on trades, prefer weak enemies already fighting, flank, seek cover, heal during downtime, contest useful drops, and navigate around structures.
- **Artillery:** Prefer mortars, keep standoff distance, estimate ballistic trajectories and avoid firing shells into buildings. More susceptible to lasers by design; pickup logic will replace a mortar rather than carry a mortar and laser together.
- **Stalker:** Prefer lightning, close range to short laser charges, pursue weakened enemies, strafe and flank.

All profiles use the same fast geometric and pathfinding foundation: terrain heightmap, building footprints, coarse A*, 3D sight lines from the muzzle, prioritized zone migration, estimated projectile avoidance, gun leading, laser charge/release timing, dynamic weapon swaps, and upgrade preferences.

They are **intentionally not identical** to your current bot, so you can test whether its policy generalizes to different opposition.

## One-time build

```bash
cd harrier_sparring_bot
go build -o harrier .
```

The compiled `harrier` binary is what the local simulator executes. **Do not copy these files into your competition team's `proj1` repository.** Create a distinct repository only if you want to publish this bot.

## Run against your existing local simulator

Your `proj1` repository already supports `ext:/absolute/path/to/binary` in `cmd/sim/main.go` through `sim.NewExtBot`. This makes integration straightforward.

Assume the package was extracted next to `proj1`, and you built `harrier` above. From the root of `proj1`:

```bash
# Set the path to wherever you extracted Harrier:
export SPAR_DIR="$(cd ../harrier_sparring_bot && pwd)"
# The recorded hello may say /arena, but the local simulator's map is here:
export SPAR_MAP_DIR="$PWD/mapdata/arena"
# Replace this path if the kit is elsewhere:
export KIT_DIR="$(cd ../mechfight-kit && pwd)"

# Exactly 1 independent Harrier against 15 copies of the original bot.
go run ./cmd/sim \
  -mode ffa -n 48 -j 2 \
  -hello "$KIT_DIR/check/stream.jsonl.gz" \
  -cand "paw,paw,paw,paw,paw,paw,paw,paw,paw,paw,paw,paw,paw,paw,paw,ext:$SPAR_DIR/harrier"
```

For **different styles in the same 16-way FFA**, use 13 copies of your main bot plus one each of Harrier, Artillery, and Stalker:

```bash
go run ./cmd/sim \
  -mode ffa -n 48 -j 2 \
  -hello "$KIT_DIR/check/stream.jsonl.gz" \
  -cand "paw,paw,paw,paw,paw,paw,paw,paw,paw,paw,paw,paw,paw,ext:$SPAR_DIR/run-harrier.sh,ext:$SPAR_DIR/run-artillery.sh,ext:$SPAR_DIR/run-stalker.sh"
```

`SPAR_MAP_DIR` is inherited by the external child process and overrides `hello.map.dir` for local simulator tests. Otherwise the bot reads the map from the official `hello.map.dir` (usually `/arena`). Native `go build` uses the host architecture (macOS or Linux); the Docker image uses linux/amd64.

**Important:** Your simulation is not the official scorer. The bot can pass the interface check without winning any FFA matches. Use multiple seeds and record placement, health, time, kill distribution and loadout outcomes before drawing conclusions. Your simulator's `-n`, `-seed`, `-trace` and `-cand` flags are useful for this.

## Docker image

```bash
docker build -t harrier-sparring:latest .
```

Launch using the normal runtime limits and with the read-only kit map mounted:

```bash
docker run --rm -i --network none --cpus 1 --memory 512m \
  --read-only --tmpfs /tmp:rw,size=64m --user 65534:65534 \
  -v "$KIT_DIR/arena:/arena:ro" \
  -e SPAR_PROFILE=artillery \
  harrier-sparring:latest
```

It waits for `hello` on stdin and returns `ready`, then one action per `obs`. Its only stdout output is valid JSON. The image does not need any network access.

## Validation

```bash
go test ./...
python3 "$KIT_DIR/check/check.py" -- env SPAR_MAP_DIR="$KIT_DIR/arena" ./harrier
```

On the development host, the 300-observation conformance stream **passed in all three profiles**: 0 late, 0 missing, 0 malformed; worst observed per-decision time across three runs was approximately 2.6 ms. This was a host-native check, **not a Docker sandbox check**; Docker is not available in this environment. Build the image on your machine and use the kit's Docker conformance command before relying on it.

## Create a separate GitHub repository (optional)

```bash
cd harrier_sparring_bot
git init
git add .
git commit -m "Independent Mech Royale sparring opponent"
gh repo create harrier-sparring-bot --public --source=. --remote=origin --push
```

The assistant did **not** make any changes to your `proj1` repository and did not create a competition submission.

## Files

- `main.go`: robust JSONL stdin/stdout loop; newest-tick behavior.
- `types.go`: protocol wire types and vector helpers.
- `world.go`: terrain, building geometry, LOS, 5m grid and bounded A*.
- `tactic.go`: target choice, danger estimation, healing, loot, zone and movement.
- `combat.go`: ballistics, weapon control, equipment and upgrade selection.
- `harrier_test.go`: unit tests for mortar fire, cook-off-safe weapon swaps, laser holding, and movement.
- `run-harrier.sh`, `run-artillery.sh`, `run-stalker.sh`: external-runner wrappers.

## Caveats

This is an original baseline sparring adversary, not a claim to be the strongest tournament bot. Its 5m ground navigation intentionally does not plan rooftop jet ascents, so roof campers can exploit it. It plans with visible/projectile observations, not omniscient enemy positions. Its `safeSpot` and firing logic are heuristic. These limitations are useful to expose *one* class of bias, but run alongside other opponent strategies too.
