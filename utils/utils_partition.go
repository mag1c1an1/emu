package utils

import (
	"emu/params"
	"log"
	"math/big"
	"strconv"
)

// Addr2Shard the default method
func Addr2Shard(addr Address) int {
	last8Addr := addr
	if len(last8Addr) > 8 {
		last8Addr = last8Addr[len(last8Addr)-8:]
	}
	num, err := strconv.ParseUint(last8Addr, 16, 64)
	if err != nil {
		log.Panic(err)
	}
	return int(num) % params.ShardNum
}

// ModBytes mod method
func ModBytes(data []byte, mod uint) uint {
	num := new(big.Int).SetBytes(data)
	result := new(big.Int).Mod(num, big.NewInt(int64(mod)))
	return uint(result.Int64())
}
