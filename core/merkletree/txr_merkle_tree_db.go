package merkletree

import (
	"encoding/binary"
	"errors"
	"sync"

	"github.com/iost-official/Go-IOS-Protocol/db/kv"
)

// TXRMerkleTreeDB is the implementation of TXRMerkleTreeDB
type TXRMerkleTreeDB struct {
	txrMerkleTreeDB *kv.Storage
}

// TXRMTDB ...
var TXRMTDB TXRMerkleTreeDB

var once sync.Once

// Uint64ToBytes ...
func Uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n)
	return b
}

// Init is init the database
func Init(LevelDBPath string) error {
	var err error
	once.Do(func() {
		levelDB, tempErr := kv.NewStorage(LevelDBPath+"TXRMerkleTreeDB", kv.LevelDBStorage)
		if tempErr != nil {
			err = errors.New("fail to init TXRMerkleTreeDB")
		}
		TXRMTDB = TXRMerkleTreeDB{levelDB}
	})
	return err
}

// Put ...
func (mdb *TXRMerkleTreeDB) Put(m *TXRMerkleTree, blockNum uint64) error {
	mByte, err := m.Encode()
	if err != nil {
		return errors.New("fail to encode TXRMerkleTree")
	}
	err = mdb.txrMerkleTreeDB.Put(Uint64ToBytes(blockNum), mByte)
	if err != nil {
		return errors.New("fail to put TXRMerkleTree")
	}
	return nil
}

// Get return the merkle tree
func (mdb *TXRMerkleTreeDB) Get(blockNum uint64) (*TXRMerkleTree, error) {
	mByte, err := mdb.txrMerkleTreeDB.Get(Uint64ToBytes(blockNum))
	if err != nil || len(mByte) == 0 {
		return nil, errors.New("fail to get TXRMerkleTree")
	}
	m := TXRMerkleTree{}
	err = m.Decode(mByte)
	if err != nil {
		return nil, errors.New("fail to decode TXRMerkleTree")
	}
	return &m, nil
}
