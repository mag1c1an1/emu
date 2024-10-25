package storage

import (
	"emu/core"
	"emu/params"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"os"
)

type Storage struct {
	dbFilePath            string // path to the database
	blockBucket           string // bucket in bolt database
	blockHeaderBucket     string // bucket in bolt database
	newestBlockHashBucket string // bucket in bolt database
	DataBase              *bolt.DB
}

func NewStorage(dbFilePath string, cc *params.ChainConfig) *Storage {
	dir := params.DatabaseWrite_path + "chainDB"
	errMkdir := os.MkdirAll(dir, os.ModePerm)
	if errMkdir != nil {
		log.Panic(errMkdir)
	}

	s := &Storage{
		dbFilePath:            dbFilePath,
		blockBucket:           "block",
		blockHeaderBucket:     "blockHeader",
		newestBlockHashBucket: "newestBlockHash",
	}
	// read + write
	db, err := bolt.Open(s.dbFilePath, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	// create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(s.blockBucket))
		if err != nil {
			log.Panic("create blocksBucket failed")
		}

		_, err = tx.CreateBucketIfNotExists([]byte(s.blockHeaderBucket))
		if err != nil {
			log.Panic("create blockHeaderBucket failed")
		}

		_, err = tx.CreateBucketIfNotExists([]byte(s.newestBlockHashBucket))
		if err != nil {
			log.Panic("create newestBlockHashBucket failed")
		}
		return nil
	})
	if err != nil {
		log.Panic("failed")
		return nil
	}
	s.DataBase = db
	return s
}

// UpdateNewestBlock update the newest block in the database
func (s *Storage) UpdateNewestBlock(newestBHash []byte) {
	err := s.DataBase.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(s.newestBlockHashBucket))
		// the bucket has the only key "OnlyNewestBlock"
		err := bucket.Put([]byte("OnlyNewestBlock"), newestBHash)
		if err != nil {
			log.Panic()
		}
		return nil
	})
	if err != nil {
		log.Panic()
	}
	fmt.Println("The newest block is updated")
}

// AddBlockHeader add a blockheader into the database
func (s *Storage) AddBlockHeader(blockHash []byte, bh *core.BlockHeader) {
	err := s.DataBase.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(s.blockHeaderBucket))
		err := bucket.Put(blockHash, bh.Encode())
		if err != nil {
			log.Panic()
		}
		return nil
	})
	if err != nil {
		log.Panic()
	}
}

// AddBlock add a block into the database
func (s *Storage) AddBlock(b *core.Block) {
	err := s.DataBase.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(s.blockBucket))
		err := bucket.Put(b.Hash, b.Encode())
		if err != nil {
			log.Panic()
		}
		return nil
	})
	if err != nil {
		log.Panic()
	}
	s.AddBlockHeader(b.Hash, b.Header)
	s.UpdateNewestBlock(b.Hash)
	fmt.Println("Block is added")
}

// GetBlockHeader read a blockheader from the database
func (s *Storage) GetBlockHeader(bHash []byte) (*core.BlockHeader, error) {
	var res *core.BlockHeader
	err := s.DataBase.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(s.blockHeaderBucket))
		bhEncoded := bucket.Get(bHash)
		if bhEncoded == nil {
			return errors.New("the block is not existed")
		}
		res = core.DecodeBH(bhEncoded)
		return nil
	})
	return res, err
}

// GetBlock read a block from the database
func (s *Storage) GetBlock(bHash []byte) (*core.Block, error) {
	var res *core.Block
	err := s.DataBase.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(s.blockBucket))
		bEncoded := bucket.Get(bHash)
		if bEncoded == nil {
			return errors.New("the block is not existed")
		}
		res = core.DecodeB(bEncoded)
		return nil
	})
	return res, err
}

// GetNewestBlockHash read the Newest block hash
func (s *Storage) GetNewestBlockHash() ([]byte, error) {
	var nhb []byte
	err := s.DataBase.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(s.newestBlockHashBucket))
		// the bucket has the only key "OnlyNewestBlock"
		nhb = bucket.Get([]byte("OnlyNewestBlock"))
		if nhb == nil {
			return errors.New("cannot find the newest block hash")
		}
		return nil
	})
	return nhb, err
}
