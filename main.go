package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type envelope struct {
	Type string `json:"type"`
	Tick int    `json:"tick"`
}
type frame struct {
	header envelope
	raw    []byte
}

func main() {
	lines := make(chan frame, 512)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 128*1024), 1024*1024)
		for scanner.Scan() {
			v := append([]byte(nil), scanner.Bytes()...)
			var e envelope
			if json.Unmarshal(v, &e) == nil {
				lines <- frame{header: e, raw: v}
			}
		}
		close(lines)
	}()
	out := bufio.NewWriterSize(os.Stdout, 64*1024)
	send := func(v any) {
		b, e := json.Marshal(v)
		if e != nil {
			return
		}
		out.Write(b)
		out.WriteByte('\n')
		out.Flush()
	}
	var pilot *Pilot
	var deferred *frame
	for {
		var f frame
		var ok bool
		if deferred != nil {
			f = *deferred
			deferred = nil
			ok = true
		} else {
			f, ok = <-lines
		}
		if !ok {
			return
		}
		switch f.header.Type {
		case "hello":
			var h Hello
			if json.Unmarshal(f.raw, &h) != nil {
				return
			}
			pilot = newPilot(h)
			send(map[string]string{"type": "ready", "name": "Harrier-" + pilot.style})
		case "end":
			return
		case "obs":
			// When multiple observations are queued, only answer the newest tick.
		drain:
			for {
				select {
				case next, ok := <-lines:
					if !ok {
						break drain
					}
					if next.header.Type != "obs" {
						deferred = &next
						break drain
					}
					if next.header.Tick >= f.header.Tick {
						f = next
					}
				default:
					break drain
				}
			}
			if pilot == nil {
				continue
			}
			var o Obs
			if err := json.Unmarshal(f.raw, &o); err != nil {
				send(Action{Tick: f.header.Tick})
				continue
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Fprintln(os.Stderr, "recovered panic:", r)
						send(Action{Tick: o.Tick})
					}
				}()
				send(pilot.act(&o))
			}()
		}
	}
}
