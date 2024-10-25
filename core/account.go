package core

import (
	"bytes"
	"crypto/sha256"
	"emu/utils"
	"encoding/gob"
	"log"
	"math/big"
)

type Account struct {
	AcAddress utils.Address
	PublicKey []byte
}

// AccountState record the details of an account, it will be saved in status trie
type AccountState struct {
	AcAddress   utils.Address // this part is not useful, abort
	Nonce       uint64
	Balance     *big.Int
	StorageRoot []byte // only for smart contract account
	CodeHash    []byte // only for smart contract account
}

// Deduct Reduce the balance of an account
func (as *AccountState) Deduct(val *big.Int) bool {
	if as.Balance.Cmp(val) < 0 {
		return false
	}
	as.Balance.Sub(as.Balance, val)
	return true
}

// Deposit Increase the balance of an account
func (as *AccountState) Deposit(value *big.Int) {
	as.Balance.Add(as.Balance, value)
}

// Encode AccountState in order to store in the MPT
func (as *AccountState) Encode() []byte {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(as)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

// DecodeAS Decode AccountState
func DecodeAS(b []byte) *AccountState {
	var as AccountState

	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&as)
	if err != nil {
		log.Panic(err)
	}
	return &as
}

// Hash AccountState for computing the MPT Root
func (as *AccountState) Hash() []byte {
	h := sha256.Sum256(as.Encode())
	return h[:]
}
