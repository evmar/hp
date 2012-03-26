hp generates graphs from google-perftools heap profiles.

`pprof`, part of [google-perftools][], does the same thing but it is
very slow for large binaries (primarily due to `addr2line` being
slow).

[google-perftools]: http://code.google.com/p/perftools/

This reimplementation has much fewer features but is also much
faster.  For more discussion, see [my blog post][blog].

[blog]: http://neugierig.org/software/blog/2012/03/heap-profiling.html

To build:

    ninja   # http://martine.github.com/ninja

or put the directory into your `GOROOT` and `go build`.

To use:

    export GOMAXPROCS=8  # number of CPUs, for multiple threads
    ./hp /path/to/binary /path/to/profile
