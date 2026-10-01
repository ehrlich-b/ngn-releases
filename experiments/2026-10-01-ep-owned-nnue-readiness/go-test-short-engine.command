env -u NGN_EP_OWNED_NNUE GOCACHE=/home/ehrli/ngn-personal-correctness-20261001/.gocache-ep-repair GOMAXPROCS=1 GOFLAGS=-p=1 /usr/local/go/bin/go test -short ./engine -count=1
