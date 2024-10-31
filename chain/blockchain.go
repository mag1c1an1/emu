package chain

import (
	"bytes"
	"emu/core"
	"emu/params"
	"emu/storage"
	"emu/utils"
	"errors"
	"fmt"
	"github.com/bits-and-blooms/bitset"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	"log"
	"math/big"
	"sync"
	"time"
)

type BlockChain struct {
	db           ethdb.Database      // the leveldb database to store in the disk, for status trie
	trieDB       *trie.Database      // the trie database which helps to store the status trie
	ChainConfig  *params.ChainConfig // the chain configuration, which can help to identify the chain
	CurrentBlock *core.Block         // the top block in this blockchain
	Storage      *storage.Storage    // Storage is the bolt-db to store the blocks
	Txpool       *core.TxPool        // the transaction pool

	pmLock       sync.RWMutex
	PartitionMap map[string]uint64 // the partition map which is defined by some algorithm can help account partition
}

// GetTxTreeRoot Get the transaction root, this root can be used to check the transactions
func GetTxTreeRoot(txs []*core.Transaction) []byte {
	// use a memory trie database to do this, instead of disk database
	trieDB := trie.NewDatabase(rawdb.NewMemoryDatabase())
	transactionTree := trie.NewEmpty(trieDB)
	for _, tx := range txs {
		_ = transactionTree.Update(tx.TxHash, []byte{0})
	}
	return transactionTree.Hash().Bytes()
}

// GetBloomFilter Get bloom filter
func GetBloomFilter(txs []*core.Transaction) *bitset.BitSet {
	bs := bitset.New(2048)
	for _, tx := range txs {
		bs.Set(utils.ModBytes(tx.TxHash, 2048))
	}
	return bs
}

// UpdatePartitionMap Write Partition Map
func (bc *BlockChain) UpdatePartitionMap(key string, val uint64) {
	bc.pmLock.Lock()
	defer bc.pmLock.Unlock()
	bc.PartitionMap[key] = val
}

// GetPartitionMap Get partition (if not exist, return default)
func (bc *BlockChain) GetPartitionMap(key string) uint64 {
	bc.pmLock.RLock()
	defer bc.pmLock.RUnlock()
	if _, ok := bc.PartitionMap[key]; !ok {
		return uint64(utils.Addr2Shard(key))
	}
	return bc.PartitionMap[key]
}

// SendTx2Pool Send a transaction to the pool (need to decide which pool should be sent)
func (bc *BlockChain) SendTx2Pool(txs []*core.Transaction) {
	bc.Txpool.AddTxs2Pool(txs)
}

// GetUpdateStatusTrie handle transactions and modify the status trie
func (bc *BlockChain) GetUpdateStatusTrie(txs []*core.Transaction) common.Hash {
	fmt.Printf("The len of txs is %d\n", len(txs))
	// the empty block (length of txs is 0) condition
	if len(txs) == 0 {
		return common.BytesToHash(bc.CurrentBlock.Header.StateRoot)
	}
	// build trie from the trieDB (in disk)
	st, err := trie.New(trie.TrieID(common.BytesToHash(bc.CurrentBlock.Header.StateRoot)), bc.trieDB)
	if err != nil {
		log.Panic(err)
	}
	cnt := 0
	// handle transactions, the signature check is ignored here
	for i, tx := range txs {
		// fmt.Printf("tx %d: %s, %s\n", i, tx.Sender, tx.Recipient)
		// senderIn := false
		if !tx.Relayed && (bc.GetPartitionMap(tx.Sender) == bc.ChainConfig.ShardID || tx.HasBroker) {
			// senderIn = true
			// fmt.Printf("the sender %s is in this shard %d, \n", tx.Sender, bc.ChainConfig.ShardId)
			// modify local account state
			sStateEnc, _ := st.Get([]byte(tx.Sender))
			var sState *core.AccountState
			if sStateEnc == nil {
				// fmt.Println("missing account SENDER, now adding account")
				ib := new(big.Int)
				ib.Add(ib, params.InitBalance)
				sState = &core.AccountState{
					Nonce:   uint64(i),
					Balance: ib,
				}
			} else {
				sState = core.DecodeAS(sStateEnc)
			}
			sBalance := sState.Balance
			if sBalance.Cmp(tx.Value) == -1 {
				fmt.Printf("the balance is less than the transfer amount\n")
				continue
			}
			sState.Deduct(tx.Value)
			_ = st.Update([]byte(tx.Sender), sState.Encode())
			cnt++
		}
		// recipientIn := false
		if bc.GetPartitionMap(tx.Recipient) == bc.ChainConfig.ShardID || tx.HasBroker {
			// fmt.Printf("the recipient %s is in this shard %d, \n", tx.Recipient, bc.ChainConfig.ShardId)
			// recipientIn = true
			// modify local state
			rStateEnc, _ := st.Get([]byte(tx.Recipient))
			var rState *core.AccountState
			if rStateEnc == nil {
				// fmt.Println("missing account RECIPIENT, now adding account")
				ib := new(big.Int)
				ib.Add(ib, params.InitBalance)
				rState = &core.AccountState{
					Nonce:   uint64(i),
					Balance: ib,
				}
			} else {
				rState = core.DecodeAS(rStateEnc)
			}
			rState.Deposit(tx.Value)
			_ = st.Update([]byte(tx.Recipient), rState.Encode())
			cnt++
		}
	}
	// commit the memory trie to the database in the disk
	if cnt == 0 {
		return common.BytesToHash(bc.CurrentBlock.Header.StateRoot)
	}
	rt, ns := st.Commit(false)
	err = bc.trieDB.Update(trie.NewWithNodeSet(ns))
	if err != nil {
		log.Panic()
	}
	err = bc.trieDB.Commit(rt, false)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("modified account number is ", cnt)
	return rt
}

// GenerateBlock generate (mine) a block, this function return a block
func (bc *BlockChain) GenerateBlock(miner int32) *core.Block {
	// pack the transactions from the txpool
	txs := bc.Txpool.PackTxs(bc.ChainConfig.BlockSize)
	bh := &core.BlockHeader{
		ParentBlockHash: bc.CurrentBlock.Hash,
		Number:          bc.CurrentBlock.Header.Number + 1,
		Time:            time.Now(),
	}
	// handle transactions to build root
	rt := bc.GetUpdateStatusTrie(txs)

	bh.StateRoot = rt.Bytes()
	bh.TxRoot = GetTxTreeRoot(txs)
	bh.Bloom = *GetBloomFilter(txs)
	// TODO change this
	bh.Miner = 0
	b := core.NewBlock(bh, txs)

	b.Hash = b.Header.Hash()
	return b
}

// NewGenesisBlock new a genesis block, this func will be invoked only once for a blockchain object
func (bc *BlockChain) NewGenesisBlock() *core.Block {
	body := make([]*core.Transaction, 0)
	bh := &core.BlockHeader{
		Number: 0,
	}
	// build a new trie database by db
	trieDB := trie.NewDatabaseWithConfig(bc.db, &trie.Config{
		Cache:     0,
		Preimages: true,
	})
	bc.trieDB = trieDB
	statusTrie := trie.NewEmpty(trieDB)
	bh.StateRoot = statusTrie.Hash().Bytes()
	bh.TxRoot = GetTxTreeRoot(body)
	bh.Bloom = *GetBloomFilter(body)
	b := core.NewBlock(bh, body)
	b.Hash = b.Header.Hash()
	return b
}

// AddGenesisBlock add the genesis block in a blockchain
func (bc *BlockChain) AddGenesisBlock(gb *core.Block) {
	bc.Storage.AddBlock(gb)
	newestHash, err := bc.Storage.GetNewestBlockHash()
	if err != nil {
		log.Panic()
	}
	curb, err := bc.Storage.GetBlock(newestHash)
	if err != nil {
		log.Panic()
	}
	bc.CurrentBlock = curb
}

// AddBlock add a block
func (bc *BlockChain) AddBlock(b *core.Block) {
	if b.Header.Number != bc.CurrentBlock.Header.Number+1 {
		fmt.Println("the block height is not correct")
		return
	}

	if !bytes.Equal(b.Header.ParentBlockHash, bc.CurrentBlock.Hash) {
		fmt.Println("err parent block hash")
		return
	}

	// if the treeRoot is existed in the node, the transactions is no need to be handled again
	_, err := trie.New(trie.TrieID(common.BytesToHash(b.Header.StateRoot)), bc.trieDB)
	if err != nil {
		rt := bc.GetUpdateStatusTrie(b.Body)
		fmt.Println(bc.CurrentBlock.Header.Number+1, "the root = ", rt.Bytes())
	}
	bc.CurrentBlock = b
	bc.Storage.AddBlock(b)
}

// NewBlockChain new a blockchain.
// the ChainConfig is pre-defined to identify the blockchain; the db is the status trie database in disk
func NewBlockChain(cc *params.ChainConfig, db ethdb.Database) (*BlockChain, error) {
	fmt.Println("Generating a new blockchain", db)
	chainDBfp := params.DatabaseWritePath + fmt.Sprintf("chainDB/S%d_N%d", cc.ShardID, cc.NodeID)
	bc := &BlockChain{
		db:           db,
		ChainConfig:  cc,
		Txpool:       core.NewTxPool(),
		Storage:      storage.NewStorage(chainDBfp, cc),
		PartitionMap: make(map[string]uint64),
	}
	curHash, err := bc.Storage.GetNewestBlockHash()
	if err != nil {
		fmt.Println("There is no existed blockchain in the database. ")
		// if the Storage bolt database cannot find the newest blockhash,
		// it means the blockchain should be built in height = 0
		if err.Error() == "cannot find the newest block hash" {
			genesisBlock := bc.NewGenesisBlock()
			bc.AddGenesisBlock(genesisBlock)
			fmt.Println("New genesis block")
			return bc, nil
		}
		log.Panic()
	}

	// there is a blockchain in the storage
	fmt.Println("Existing blockchain found")
	curb, err := bc.Storage.GetBlock(curHash)
	if err != nil {
		log.Panic()
	}

	bc.CurrentBlock = curb
	trieDB := trie.NewDatabaseWithConfig(db, &trie.Config{
		Cache:     0,
		Preimages: true,
	})
	bc.trieDB = trieDB
	// check the existence of the trie database
	_, err = trie.New(trie.TrieID(common.BytesToHash(curb.Header.StateRoot)), trieDB)
	if err != nil {
		log.Panic()
	}
	fmt.Println("The status trie can be built")
	fmt.Println("Generated a new blockchain successfully")
	return bc, nil
}

// IsValidBlock check a block is valid or not in this blockchain config
func (bc *BlockChain) IsValidBlock(b *core.Block) error {
	if string(b.Header.ParentBlockHash) != string(bc.CurrentBlock.Hash) {
		fmt.Println("the parent block hash is not equal to the current block hash")
		return errors.New("the parent block hash is not equal to the current block hash")
	} else if string(GetTxTreeRoot(b.Body)) != string(b.Header.TxRoot) {
		fmt.Println("the transaction root is wrong")
		return errors.New("the transaction root is wrong")
	}
	return nil
}

// AddAccounts add accounts
func (bc *BlockChain) AddAccounts(ac []string, as []*core.AccountState, miner int32) {
	fmt.Printf("The len of accounts is %d, now adding the accounts\n", len(ac))

	bh := &core.BlockHeader{
		ParentBlockHash: bc.CurrentBlock.Hash,
		Number:          bc.CurrentBlock.Header.Number + 1,
		Time:            time.Time{},
	}
	// handle transactions to build root
	rt := bc.CurrentBlock.Header.StateRoot
	if len(ac) != 0 {
		st, err := trie.New(trie.TrieID(common.BytesToHash(bc.CurrentBlock.Header.StateRoot)), bc.trieDB)
		if err != nil {
			log.Panic(err)
		}
		for i, addr := range ac {
			if bc.GetPartitionMap(addr) == bc.ChainConfig.ShardID {
				ib := new(big.Int)
				ib.Add(ib, as[i].Balance)
				newState := &core.AccountState{
					Balance: ib,
					Nonce:   as[i].Nonce,
				}
				err := st.Update([]byte(addr), newState.Encode())
				if err != nil {
					log.Panic(err)
					return
				}
			}
		}
		rrt, ns := st.Commit(false)
		err = bc.trieDB.Update(trie.NewWithNodeSet(ns))
		if err != nil {
			log.Panic(err)
		}
		err = bc.trieDB.Commit(rrt, false)
		if err != nil {
			log.Panic(err)
		}
		rt = rrt.Bytes()
	}

	emptyTxs := make([]*core.Transaction, 0)
	bh.StateRoot = rt
	bh.TxRoot = GetTxTreeRoot(emptyTxs)
	bh.Bloom = *GetBloomFilter(emptyTxs)
	bh.Miner = 0
	b := core.NewBlock(bh, emptyTxs)
	b.Hash = b.Header.Hash()

	bc.CurrentBlock = b
	bc.Storage.AddBlock(b)
}

// FetchAccounts fetch accounts
func (bc *BlockChain) FetchAccounts(addrs []string) []*core.AccountState {
	res := make([]*core.AccountState, 0)
	st, err := trie.New(trie.TrieID(common.BytesToHash(bc.CurrentBlock.Header.StateRoot)), bc.trieDB)
	if err != nil {
		log.Panic(err)
	}
	for _, addr := range addrs {
		asEnc, _ := st.Get([]byte(addr))
		var stateA *core.AccountState
		if asEnc == nil {
			ib := new(big.Int)
			ib.Add(ib, params.InitBalance)
			stateA = &core.AccountState{
				Nonce:   uint64(0),
				Balance: ib,
			}
		} else {
			stateA = core.DecodeAS(asEnc)
		}
		res = append(res, stateA)
	}
	return res
}

// CloseBlockChain close a blockChain, close the database interfaces
func (bc *BlockChain) CloseBlockChain() {
	err := bc.Storage.DataBase.Close()
	if err != nil {
		log.Panic(err)
		return
	}
	err = bc.trieDB.CommitPreimages()
	if err != nil {
		log.Panic(err)
		return
	}
	err = bc.db.Close()
	if err != nil {
		log.Panic(err)
		return
	}
}

// PrintBlockChain print the details of a blockchain
func (bc *BlockChain) PrintBlockChain() string {
	vals := []interface{}{
		bc.CurrentBlock.Header.Number,
		bc.CurrentBlock.Hash,
		bc.CurrentBlock.Header.StateRoot,
		bc.CurrentBlock.Header.Time,
		bc.trieDB,
		// len(bc.Txpool.RelayPool[1]),
	}
	res := fmt.Sprintf("%v\n", vals)
	fmt.Println(res)
	return res
}
