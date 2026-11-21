# Harrier — independent Mech Royale sparring bot

An original, standalone Go opponent for the Builders **Mech Royale** 16-player free-for-all. This is a **sparring partner only**: it is not submitted to any team's scored entry, and it is unrelated to `shaunkar1/proj1`.

## Quick start

If the individual Go files are not yet visible, extract `harrier_sparring_bot.zip` first:

```sh
unzip -oq harrier_sparring_bot.zip
cd harrier_sparring_bot
go test ./...
go build -o harrier .
```

Once the source files have been published at the repository root:

```sh
go test ./...
go build -o harrier .
docker build -t harrier-sparring .
```

The bot speaks **mechfight/1 JSON lines** via stdin/stdout. Set `SPAR_PROFILE=harrier`, `artillery`, or `stalker` to select an independent opponent strategy. The Go binary can be used with the existing simulator's `ext:/absolute/path/to/harrier` integration.

The source archive includes detailed simulation commands and validation notes. The protocol conformance test passed locally: 300/300 timely replies. Docker-sandbox conformance and win rate have not been verified.
