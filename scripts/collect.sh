#!/bin/bash
# Collect debug data.
# To run with edgevpn, run with --debug and --api flags on.
# Note: requires https://github.com/whyrusleeping/stackparse for parsing goroutine debug stacks

mkdir collect
((count=1))
while true; do
	(( count = count + 1))
	curl http://localhost:8080/debug/pprof/heap > collect/heap$count
	curl 'http://localhost:8080/debug/pprof/goroutine' > collect/goroutine$count
	curl 'http://localhost:8080/debug/pprof/goroutine?debug=2' > collect/goroutine_debug_$count
	stackparse --summary collect/goroutine_debug_$count > collect/goroutine_debug_${count}_summary
	sleep 60
done
	
