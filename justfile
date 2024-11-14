build:
    go build

sv:
    go run main.go -c -S 1 -N 4

gen NUM:
    ./emu -g -f -S 1 -N {{NUM}}

killall:
    killall -9 emu

clean:
    -rm run_IpAddr=127_0_0_1.sh
    -rm -rf expTest

ls:
    ps aux | rg emu


iptable NUM:
    python3 scripts/create_iptable.py {{NUM}}

pre NUM: clean build
    just iptable {{NUM}}
    just gen {{NUM}}

build_runner:
    cd runner && cargo build --release
    cp ./runner/target/release/runner ./bin/runner

start:
    ./bin/runner run_IpAddr=127_0_0_1.sh
