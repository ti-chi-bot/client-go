// Copyright 2021 TiKV Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// NOTE: The code in this file is based on code from the
// TiDB project, licensed under the Apache License v 2.0
//
// https://github.com/pingcap/tidb/tree/cc5e161ac06827589c4966674597c137cc9e809c/store/tikv/unionstore/memdb_test.go
//

// Copyright 2020 PingCAP, Inc.
//
// Copyright 2015 Wenbin Xiao
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package unionstore

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	leveldb "github.com/pingcap/goleveldb/leveldb/memdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikv/client-go/v2/kv"
)

type KeyFlags = kv.KeyFlags

func TestGetSet(t *testing.T) {
	testGetSet(t, newRbtDBWithContext())
}

func testGetSet(t *testing.T, db MemBuffer) {
	require := require.New(t)

	const cnt = 10000
	fillDB(db, cnt)

	var buf [4]byte
	for i := 0; i < cnt; i++ {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		v, err := db.Get(context.Background(), buf[:])
		require.Nil(err)
		require.Equal(v, buf[:])
	}
}

func TestIterator(t *testing.T) {
	testIterator(t, newRbtDBWithContext())
}

func testIterator(t *testing.T, db MemBuffer) {
	assert := assert.New(t)
	const cnt = 10000
	fillDB(db, cnt)

	var buf [4]byte
	var i int

	for it, _ := db.Iter(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i++
	}
	assert.Equal(i, cnt)

	for it, _ := db.IterReverse(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i-1))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i--
	}
	assert.Equal(i, 0)

	var upperBoundBytes, lowerBoundBytes [4]byte
	bound := 400
	binary.BigEndian.PutUint32(upperBoundBytes[:], uint32(bound))
	for it, _ := db.Iter(nil, upperBoundBytes[:]); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i++
	}
	assert.Equal(i, bound)

	i = cnt
	binary.BigEndian.PutUint32(lowerBoundBytes[:], uint32(bound))
	for it, _ := db.IterReverse(nil, lowerBoundBytes[:]); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i-1))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i--
	}
	assert.Equal(i, bound)
}

func TestDiscard(t *testing.T) {
	testDiscard(t, newRbtDBWithContext())
}

func testDiscard(t *testing.T, db MemBuffer) {
	assert := assert.New(t)

	const cnt = 10000
	base := deriveAndFill(db, 0, cnt, 0)
	sz := db.Size()

	db.Cleanup(deriveAndFill(db, 0, cnt, 1))
	assert.Equal(db.Len(), cnt)
	assert.Equal(db.Size(), sz)

	var buf [4]byte

	for i := 0; i < cnt; i++ {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		v, err := db.Get(context.Background(), buf[:])
		assert.Nil(err)
		assert.Equal(v, buf[:])
	}

	var i int
	for it, _ := db.Iter(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i++
	}
	assert.Equal(i, cnt)

	i--
	for it, _ := db.IterReverse(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		assert.Equal(it.Value(), buf[:])
		i--
	}
	assert.Equal(i, -1)

	db.Cleanup(base)
	for i := 0; i < cnt; i++ {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		_, err := db.Get(context.Background(), buf[:])
		assert.NotNil(err)
	}
	it, _ := db.Iter(nil, nil)
	assert.False(it.Valid())
}

func TestFlushOverwrite(t *testing.T) {
	testFlushOverwrite(t, newRbtDBWithContext())
}

func testFlushOverwrite(t *testing.T, db MemBuffer) {
	assert := assert.New(t)

	const cnt = 10000
	db.Release(deriveAndFill(db, 0, cnt, 0))
	sz := db.Size()

	db.Release(deriveAndFill(db, 0, cnt, 1))

	assert.Equal(db.Len(), cnt)
	assert.Equal(db.Size(), sz)

	var kbuf, vbuf [4]byte

	for i := 0; i < cnt; i++ {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		v, err := db.Get(context.Background(), kbuf[:])
		assert.Nil(err)
		assert.Equal(v, vbuf[:])
	}

	var i int
	for it, _ := db.Iter(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		assert.Equal(it.Key(), kbuf[:])
		assert.Equal(it.Value(), vbuf[:])
		i++
	}
	assert.Equal(i, cnt)

	i--
	for it, _ := db.IterReverse(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		assert.Equal(it.Key(), kbuf[:])
		assert.Equal(it.Value(), vbuf[:])
		i--
	}
	assert.Equal(i, -1)
}

func TestComplexUpdate(t *testing.T) {
	testComplexUpdate(t, newRbtDBWithContext())
}

func testComplexUpdate(t *testing.T, db MemBuffer) {
	assert := assert.New(t)

	const (
		keep      = 3000
		overwrite = 6000
		insert    = 9000
	)

	db.Release(deriveAndFill(db, 0, overwrite, 0))
	assert.Equal(db.Len(), overwrite)
	db.Release(deriveAndFill(db, keep, insert, 1))
	assert.Equal(db.Len(), insert)

	var kbuf, vbuf [4]byte

	for i := 0; i < insert; i++ {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i))
		if i >= keep {
			binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		}
		v, err := db.Get(context.Background(), kbuf[:])
		assert.Nil(err)
		assert.Equal(v, vbuf[:])
	}
}

func TestNestedSandbox(t *testing.T) {
	testNestedSandbox(t, newRbtDBWithContext())
}

func testNestedSandbox(t *testing.T, db MemBuffer) {
	assert := assert.New(t)
	h0 := deriveAndFill(db, 0, 200, 0)
	h1 := deriveAndFill(db, 0, 100, 1)
	h2 := deriveAndFill(db, 50, 150, 2)
	h3 := deriveAndFill(db, 100, 120, 3)
	h4 := deriveAndFill(db, 0, 150, 4)
	db.Cleanup(h4) // Discard (0..150 -> 4)
	db.Release(h3) // Flush (100..120 -> 3)
	db.Cleanup(h2) // Discard (100..120 -> 3) & (50..150 -> 2)
	db.Release(h1) // Flush (0..100 -> 1)
	db.Release(h0) // Flush (0..100 -> 1) & (0..200 -> 0)
	// The final result should be (0..100 -> 1) & (101..200 -> 0)

	var kbuf, vbuf [4]byte

	for i := 0; i < 200; i++ {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i))
		if i < 100 {
			binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		}
		v, err := db.Get(context.Background(), kbuf[:])
		assert.Nil(err)
		assert.Equal(v, vbuf[:])
	}

	var i int

	for it, _ := db.Iter(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i))
		if i < 100 {
			binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		}
		assert.Equal(it.Key(), kbuf[:])
		assert.Equal(it.Value(), vbuf[:])
		i++
	}
	assert.Equal(i, 200)

	i--
	for it, _ := db.IterReverse(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i))
		if i < 100 {
			binary.BigEndian.PutUint32(vbuf[:], uint32(i+1))
		}
		assert.Equal(it.Key(), kbuf[:])
		assert.Equal(it.Value(), vbuf[:])
		i--
	}
	assert.Equal(i, -1)
}

func TestOverwrite(t *testing.T) {
	testOverwrite(t, newRbtDBWithContext())
}

func testOverwrite(t *testing.T, db MemBuffer) {
	assert := assert.New(t)

	const cnt = 10000
	fillDB(db, cnt)
	var buf [4]byte

	sz := db.Size()
	for i := 0; i < cnt; i += 3 {
		var newBuf [4]byte
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		binary.BigEndian.PutUint32(newBuf[:], uint32(i*10))
		db.Set(buf[:], newBuf[:])
	}
	assert.Equal(db.Len(), cnt)
	assert.Equal(db.Size(), sz)

	for i := 0; i < cnt; i++ {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		val, _ := db.Get(context.Background(), buf[:])
		v := binary.BigEndian.Uint32(val)
		if i%3 == 0 {
			assert.Equal(v, uint32(i*10))
		} else {
			assert.Equal(v, uint32(i))
		}
	}

	var i int

	for it, _ := db.Iter(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		v := binary.BigEndian.Uint32(it.Value())
		if i%3 == 0 {
			assert.Equal(v, uint32(i*10))
		} else {
			assert.Equal(v, uint32(i))
		}
		i++
	}
	assert.Equal(i, cnt)

	i--
	for it, _ := db.IterReverse(nil, nil); it.Valid(); it.Next() {
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		assert.Equal(it.Key(), buf[:])
		v := binary.BigEndian.Uint32(it.Value())
		if i%3 == 0 {
			assert.Equal(v, uint32(i*10))
		} else {
			assert.Equal(v, uint32(i))
		}
		i--
	}
	assert.Equal(i, -1)
}

func TestReset(t *testing.T) {
	testReset(t, newRbtDBWithContext())
}

func testReset(t *testing.T, db interface {
	MemBuffer
	Reset()
}) {
	assert := assert.New(t)
	fillDB(db, 1000)
	db.Reset()
	_, err := db.Get(context.Background(), []byte{0, 0, 0, 0})
	assert.NotNil(err)
	it, _ := db.Iter(nil, nil)
	assert.False(it.Valid())
}

func TestInspectStage(t *testing.T) {
	testInspectStage(t, newRbtDBWithContext())
}

func testInspectStage(t *testing.T, db MemBuffer) {
	assert := assert.New(t)

	h1 := deriveAndFill(db, 0, 1000, 0)
	h2 := deriveAndFill(db, 500, 1000, 1)
	for i := 500; i < 1500; i++ {
		var kbuf [4]byte
		// don't update in place
		var vbuf [5]byte
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i+2))
		db.Set(kbuf[:], vbuf[:])
	}
	h3 := deriveAndFill(db, 1000, 2000, 3)

	db.InspectStage(h3, func(key []byte, _ KeyFlags, val []byte) {
		k := int(binary.BigEndian.Uint32(key))
		v := int(binary.BigEndian.Uint32(val))

		assert.True(k >= 1000 && k < 2000)
		assert.Equal(v-k, 3)
	})

	db.InspectStage(h2, func(key []byte, _ KeyFlags, val []byte) {
		k := int(binary.BigEndian.Uint32(key))
		v := int(binary.BigEndian.Uint32(val))

		assert.True(k >= 500 && k < 2000)
		if k < 1000 {
			assert.Equal(v-k, 2)
		} else {
			assert.Equal(v-k, 3)
		}
	})

	db.Cleanup(h3)
	db.Release(h2)

	db.InspectStage(h1, func(key []byte, _ KeyFlags, val []byte) {
		k := int(binary.BigEndian.Uint32(key))
		v := int(binary.BigEndian.Uint32(val))

		assert.True(k >= 0 && k < 1500)
		if k < 500 {
			assert.Equal(v-k, 0)
		} else {
			assert.Equal(v-k, 2)
		}
	})

	db.Release(h1)
}

func TestDirty(t *testing.T) {
	testDirty(t, func() MemBuffer { return newRbtDBWithContext() })
}

func testDirty(t *testing.T, createDb func() MemBuffer) {
	assert := assert.New(t)

	db := createDb()
	db.Set([]byte{1}, []byte{1})
	assert.True(db.Dirty())

	db = createDb()
	h := db.Staging()
	db.Set([]byte{1}, []byte{1})
	db.Cleanup(h)
	assert.False(db.Dirty())

	h = db.Staging()
	db.Set([]byte{1}, []byte{1})
	db.Release(h)
	assert.True(db.Dirty())

	// persistent flags will make memdb dirty.
	db = createDb()
	h = db.Staging()
	db.SetWithFlags([]byte{1}, []byte{1}, kv.SetKeyLocked)
	db.Cleanup(h)
	assert.True(db.Dirty())

	// non-persistent flags will not make memdb dirty.
	db = createDb()
	h = db.Staging()
	db.SetWithFlags([]byte{1}, []byte{1}, kv.SetPresumeKeyNotExists)
	db.Cleanup(h)
	assert.False(db.Dirty())
}

func TestFlags(t *testing.T) {
	testFlags(t, newRbtDBWithContext(), func(db MemBuffer) Iterator { return db.(*rbtDBWithContext).IterWithFlags(nil, nil) })
}

func testFlags(t *testing.T, db MemBuffer, iterWithFlags func(db MemBuffer) Iterator) {
	assert := assert.New(t)

	const cnt = 10000
	h := db.Staging()
	for i := uint32(0); i < cnt; i++ {
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], i)
		if i%2 == 0 {
			db.SetWithFlags(buf[:], buf[:], kv.SetPresumeKeyNotExists, kv.SetKeyLocked)
		} else {
			db.SetWithFlags(buf[:], buf[:], kv.SetPresumeKeyNotExists)
		}
	}
	db.Cleanup(h)

	for i := uint32(0); i < cnt; i++ {
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], i)
		_, err := db.Get(context.Background(), buf[:])
		assert.NotNil(err)
		flags, err := db.GetFlags(buf[:])
		if i%2 == 0 {
			assert.Nil(err)
			assert.True(flags.HasLocked())
			assert.False(flags.HasPresumeKeyNotExists())
		} else {
			assert.NotNil(err)
		}
	}

	assert.Equal(db.Len(), 5000)
	assert.Equal(db.Size(), 20000)

	it, _ := db.Iter(nil, nil)
	assert.False(it.Valid())

	it = iterWithFlags(db)
	for ; it.Valid(); it.Next() {
		k := binary.BigEndian.Uint32(it.Key())
		assert.True(k%2 == 0)
	}

	for i := uint32(0); i < cnt; i++ {
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], i)
		db.UpdateFlags(buf[:], kv.DelKeyLocked)
	}
	for i := uint32(0); i < cnt; i++ {
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], i)
		_, err := db.Get(context.Background(), buf[:])
		assert.NotNil(err)

		// UpdateFlags will create missing node.
		flags, err := db.GetFlags(buf[:])
		assert.Nil(err)
		assert.False(flags.HasLocked())
	}
}

func checkConsist(t *testing.T, p1 *MemDB, p2 *leveldb.DB) {
	assert := assert.New(t)

	assert.Equal(p1.Len(), p2.Len())
	assert.Equal(p1.Size(), p2.Size())

	it1, _ := p1.Iter(nil, nil)
	it2 := p2.NewIterator(nil)

	var prevKey, prevVal []byte
	for it2.First(); it2.Valid(); it2.Next() {
		v, err := p1.Get(context.Background(), it2.Key())
		assert.Nil(err)
		assert.Equal(v, it2.Value())

		assert.Equal(it1.Key(), it2.Key())
		assert.Equal(it1.Value(), it2.Value())

		it, _ := p1.Iter(it2.Key(), nil)
		assert.Equal(it.Key(), it2.Key())
		assert.Equal(it.Value(), it2.Value())

		if prevKey != nil {
			it, _ = p1.IterReverse(it2.Key(), nil)
			assert.Equal(it.Key(), prevKey)
			assert.Equal(it.Value(), prevVal)
		}

		it1.Next()
		prevKey = it2.Key()
		prevVal = it2.Value()
	}

	it1, _ = p1.IterReverse(nil, nil)
	for it2.Last(); it2.Valid(); it2.Prev() {
		assert.Equal(it1.Key(), it2.Key())
		assert.Equal(it1.Value(), it2.Value())
		it1.Next()
	}
}

func fillDB(db MemBuffer, cnt int) {
	h := deriveAndFill(db, 0, cnt, 0)
	db.Release(h)
}

func deriveAndFill(db MemBuffer, start, end, valueBase int) int {
	h := db.Staging()
	var kbuf, vbuf [4]byte
	for i := start; i < end; i++ {
		binary.BigEndian.PutUint32(kbuf[:], uint32(i))
		binary.BigEndian.PutUint32(vbuf[:], uint32(i+valueBase))
		db.Set(kbuf[:], vbuf[:])
	}
	return h
}

const (
	startIndex = 0
	testCount  = 2
	indexStep  = 2
)

func insertData(t *testing.T, buffer *MemDB) {
	for i := startIndex; i < testCount; i++ {
		val := encodeInt(i * indexStep)
		err := buffer.Set(val, val)
		assert.Nil(t, err)
	}
}

func encodeInt(n int) []byte {
	return []byte(fmt.Sprintf("%010d", n))
}

func decodeInt(s []byte) int {
	var n int
	fmt.Sscanf(string(s), "%010d", &n)
	return n
}

func valToStr(iter Iterator) string {
	val := iter.Value()
	return string(val)
}

func checkNewIterator(t *testing.T, buffer *MemDB) {
	assert := assert.New(t)
	for i := startIndex; i < testCount; i++ {
		val := encodeInt(i * indexStep)
		iter, err := buffer.Iter(val, nil)
		assert.Nil(err)
		assert.Equal(iter.Key(), val)
		assert.Equal(decodeInt([]byte(valToStr(iter))), i*indexStep)
		iter.Close()
	}

	// Test iterator Next()
	for i := startIndex; i < testCount-1; i++ {
		val := encodeInt(i * indexStep)
		iter, err := buffer.Iter(val, nil)
		assert.Nil(err)
		assert.Equal(iter.Key(), val)
		assert.Equal(valToStr(iter), string(val))

		err = iter.Next()
		assert.Nil(err)
		assert.True(iter.Valid())

		val = encodeInt((i + 1) * indexStep)
		assert.Equal(iter.Key(), val)
		assert.Equal(valToStr(iter), string(val))
		iter.Close()
	}

	// Non exist and beyond maximum seek test
	iter, err := buffer.Iter(encodeInt(testCount*indexStep), nil)
	assert.Nil(err)
	assert.False(iter.Valid())

	// Non exist but between existing keys seek test,
	// it returns the smallest key that larger than the one we are seeking
	inBetween := encodeInt((testCount-1)*indexStep - 1)
	last := encodeInt((testCount - 1) * indexStep)
	iter, err = buffer.Iter(inBetween, nil)
	assert.Nil(err)
	assert.True(iter.Valid())
	assert.NotEqual(iter.Key(), inBetween)
	assert.Equal(iter.Key(), last)
	iter.Close()
}

func mustGet(t *testing.T, buffer *MemDB) {
	for i := startIndex; i < testCount; i++ {
		s := encodeInt(i * indexStep)
		val, err := buffer.Get(context.Background(), s)
		assert.Nil(t, err)
		assert.Equal(t, string(val), string(s))
	}
}

func TestKVGetSet(t *testing.T) {
	buffer := NewMemDB()
	insertData(t, buffer)
	mustGet(t, buffer)
}

func TestNewIterator(t *testing.T) {
	assert := assert.New(t)
	buffer := NewMemDB()
	// should be invalid
	iter, err := buffer.Iter(nil, nil)
	assert.Nil(err)
	assert.False(iter.Valid())

	insertData(t, buffer)
	checkNewIterator(t, buffer)
}

// FnKeyCmp is the function for iterator the keys
type FnKeyCmp func(key []byte) bool

// NextUntil applies FnKeyCmp to each entry of the iterator until meets some condition.
// It will stop when fn returns true, or iterator is invalid or an error occurs.
func NextUntil(it Iterator, fn FnKeyCmp) error {
	var err error
	for it.Valid() && !fn(it.Key()) {
		err = it.Next()
		if err != nil {
			return err
		}
	}
	return nil
}

func TestIterNextUntil(t *testing.T) {
	assert := assert.New(t)
	buffer := NewMemDB()
	insertData(t, buffer)

	iter, err := buffer.Iter(nil, nil)
	assert.Nil(err)

	err = NextUntil(iter, func(k []byte) bool {
		return false
	})
	assert.Nil(err)
	assert.False(iter.Valid())
}

func TestBasicNewIterator(t *testing.T) {
	assert := assert.New(t)
	buffer := NewMemDB()
	it, err := buffer.Iter([]byte("2"), nil)
	assert.Nil(err)
	assert.False(it.Valid())
}

func TestNewIteratorMin(t *testing.T) {
	assert := assert.New(t)
	kvs := []struct {
		key   string
		value string
	}{
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000001", "lock-version"},
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000001_0002", "1"},
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000001_0003", "hello"},
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000002", "lock-version"},
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000002_0002", "2"},
		{"DATA_test_main_db_tbl_tbl_test_record__00000000000000000002_0003", "hello"},
	}
	buffer := NewMemDB()
	for _, kv := range kvs {
		err := buffer.Set([]byte(kv.key), []byte(kv.value))
		assert.Nil(err)
	}

	cnt := 0
	it, err := buffer.Iter(nil, nil)
	assert.Nil(err)
	for it.Valid() {
		cnt++
		err := it.Next()
		assert.Nil(err)
	}
	assert.Equal(cnt, 6)

	it, err = buffer.Iter([]byte("DATA_test_main_db_tbl_tbl_test_record__00000000000000000000"), nil)
	assert.Nil(err)
	assert.Equal(string(it.Key()), "DATA_test_main_db_tbl_tbl_test_record__00000000000000000001")
}

func TestMemDBStaging(t *testing.T) {
	assert := assert.New(t)
	buffer := NewMemDB()
	err := buffer.Set([]byte("x"), make([]byte, 2))
	assert.Nil(err)

	h1 := buffer.Staging()
	err = buffer.Set([]byte("x"), make([]byte, 3))
	assert.Nil(err)

	h2 := buffer.Staging()
	err = buffer.Set([]byte("yz"), make([]byte, 1))
	assert.Nil(err)

	v, _ := buffer.Get(context.Background(), []byte("x"))
	assert.Equal(len(v), 3)

	buffer.Release(h2)

	v, _ = buffer.Get(context.Background(), []byte("yz"))
	assert.Equal(len(v), 1)

	buffer.Cleanup(h1)

	v, _ = buffer.Get(context.Background(), []byte("x"))
	assert.Equal(len(v), 2)
}

func TestBufferLimit(t *testing.T) {
	testBufferLimit(t, newRbtDBWithContext())
}

func testBufferLimit(t *testing.T, buffer MemBuffer) {
	assert := assert.New(t)
	buffer.SetEntrySizeLimit(500, 1000)

	err := buffer.Set([]byte("x"), make([]byte, 500))
	assert.NotNil(err) // entry size limit

	err = buffer.Set([]byte("x"), make([]byte, 499))
	assert.Nil(err)
	err = buffer.Set([]byte("yz"), make([]byte, 499))
	assert.NotNil(err) // buffer size limit

	err = buffer.Delete(make([]byte, 499))
	assert.Nil(err)

	err = buffer.Delete(make([]byte, 500))
	assert.NotNil(err)
}

func TestUnsetTemporaryFlag(t *testing.T) {
	require := require.New(t)
	db := NewMemDB()
	key := []byte{1}
	value := []byte{2}
	db.SetWithFlags(key, value, kv.SetNeedConstraintCheckInPrewrite)
	db.Set(key, value)
	flags, err := db.GetFlags(key)
	require.Nil(err)
	require.False(flags.HasNeedConstraintCheckInPrewrite())
}

func TestSnapshotGetIter(t *testing.T) {
	assert := assert.New(t)
	buffer := NewMemDB()
	var getters []Getter
	var iters []Iterator
	for i := 0; i < 100; i++ {
		assert.Nil(buffer.Set([]byte{byte(0)}, []byte{byte(i)}))
		// getter
		getter := buffer.SnapshotGetter()
		val, err := getter.Get(context.Background(), []byte{byte(0)})
		assert.Nil(err)
		assert.Equal(val, []byte{byte(min(i, 50))})
		getters = append(getters, getter)
		// iter
		iter := buffer.SnapshotIter(nil, nil)
		assert.Nil(err)
		assert.Equal(iter.Key(), []byte{byte(0)})
		assert.Equal(iter.Value(), []byte{byte(min(i, 50))})
		iter.Close()
		iters = append(iters, buffer.SnapshotIter(nil, nil))
		if i == 50 {
			_ = buffer.Staging()
		}
	}
	for _, getter := range getters {
		val, err := getter.Get(context.Background(), []byte{byte(0)})
		assert.Nil(err)
		assert.Equal(val, []byte{byte(50)})
	}
	for _, iter := range iters {
		assert.Equal(iter.Key(), []byte{byte(0)})
		assert.Equal(iter.Value(), []byte{byte(50)})
	}
}
