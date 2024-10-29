/*
Copyright 2024 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"sync"
	"time"
)

const (
	// noExpiration is a sentinel value used to indicate no expiration time.
	// It is used instead of 0, to be able to sort items by expiration time ascending.
	noExpiration = time.Second * 86400 * 365 * 10 // 10 years
	// defaultInterval is the default interval for the janitor to run.
	defaultInterval = time.Minute
)

// Cache[T] is a thread-safe in-memory key/object store.
// It can be used to store objects with optional expiration.
// A function to extract the key from the object must be provided.
// Use the New function to create a new cache that is ready to use.
type Cache[T any] struct {
	*cache[T]
	// keyFunc is used to make the key for objects stored in and retrieved from index, and
	// should be deterministic.
	keyFunc KeyFunc[T]
}

// item is an item stored in the cache.
type item[T any] struct {
	key string
	// object is the item's object.
	object T
	// expiresAt is the item's expiration time.
	expiresAt time.Time
}

type cache[T any] struct {
	// index holds the cache index.
	index map[string]*item[T]
	// items is the store of elements in the cache.
	items []*item[T]

	// capacity is the maximum number of index the cache can hold.
	capacity     int
	metrics      *cacheMetrics
	labelsFunc   GetLvsFunc[T]
	janitor      *janitor[T]
	snapshotPath string
	buf          buffer
	// sorted indicates whether the items are sorted by expiration time.
	// It is initially true, and set to false when the items are not sorted.
	sorted bool
	closed bool

	mu sync.RWMutex
}

var _ Expirable[any] = &Cache[any]{}
var _ Persistable = &Cache[any]{}

// New creates a new cache with the given configuration.
func New[T any](capacity int, keyFunc KeyFunc[T], opts ...Options[T]) (*Cache[T], error) {
	opt, err := makeOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to apply options: %w", err)
	}

	c := &cache[T]{
		index:        make(map[string]*item[T]),
		items:        make([]*item[T], 0, capacity),
		sorted:       true,
		capacity:     capacity,
		snapshotPath: opt.snapshotPath,
		labelsFunc:   opt.labelsFunc,
		janitor: &janitor[T]{
			interval: opt.interval,
			stop:     make(chan bool),
		},
	}

	if opt.registerer != nil {
		c.metrics = newCacheMetrics(opt.registerer, opt.extraLabels...)
	}

	C := &Cache[T]{cache: c, keyFunc: keyFunc}

	if c.snapshotPath != "" {
		// load the cache from the file if it exists
		if _, err := os.Stat(c.snapshotPath); err == nil {
			err = c.load()
			if err != nil {
				return nil, err
			}
		}
	}

	if opt.interval > 0 {
		go c.janitor.run(c)
	}

	return C, nil
}

func makeOptions[T any](opts ...Options[T]) (*storeOptions[T], error) {
	opt := storeOptions[T]{}
	for _, o := range opts {
		err := o(&opt)
		if err != nil {
			return nil, err
		}
	}
	if opt.interval <= 0 {
		opt.interval = defaultInterval
	}
	return &opt, nil
}

// Close closes the cache. It also stops the expiration eviction process.
func (c *Cache[T]) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrCacheClosed
	}
	c.janitor.stop <- true
	c.closed = true
	return nil
}

// Set an item in the cache, existing index will be overwritten.
// If the cache is full, Add will return an error.
func (c *Cache[T]) Set(object T) error {
	key, err := c.keyFunc(object)
	if err != nil {
		recordRequest(c.metrics, StatusFailure)
		return &CacheError{Reason: ErrInvalidKey, Err: err}
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusFailure)
		return ErrCacheClosed
	}
	_, found := c.index[key]
	if found {
		c.set(key, object)
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		return nil
	}

	if c.capacity > 0 && len(c.index) < c.capacity {
		c.set(key, object)
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		recordItemIncrement(c.metrics)
		return nil
	}
	c.mu.Unlock()
	recordRequest(c.metrics, StatusFailure)
	return ErrCacheFull
}

func (c *cache[T]) set(key string, object T) {
	item := item[T]{
		key:       key,
		object:    object,
		expiresAt: time.Now().Add(noExpiration),
	}

	if _, found := c.index[key]; found {
		// item already exists, update it only
		c.index[key] = &item
		return
	}
	c.index[key] = &item
	c.items = append(c.items, &item)
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *Cache[T]) Get(object T) (item T, exists bool, err error) {
	var res T
	lvs := []string{}
	if c.labelsFunc != nil {
		lvs, err = c.labelsFunc(object, len(c.metrics.getExtraLabels()))
		if err != nil {
			recordRequest(c.metrics, StatusFailure)
			return res, false, &CacheError{Reason: ErrInvalidLabels, Err: err}
		}
	}
	key, err := c.keyFunc(object)
	if err != nil {
		recordRequest(c.metrics, StatusFailure)
		return res, false, &CacheError{Reason: ErrInvalidKey, Err: err}
	}
	item, found, err := c.get(key)
	if err != nil {
		return res, false, err
	}
	if !found {
		recordEvent(c.metrics, CacheEventTypeMiss, lvs...)
		return res, false, nil
	}
	recordEvent(c.metrics, CacheEventTypeHit, lvs...)
	return item, true, nil
}

// GetByKey returns the object for the given key.
func (c *Cache[T]) GetByKey(key string) (T, bool, error) {
	var res T
	index, found, err := c.get(key)
	if err != nil {
		return res, false, err
	}
	if !found {
		recordEvent(c.metrics, CacheEventTypeMiss)
		return res, false, nil
	}

	recordEvent(c.metrics, CacheEventTypeHit)
	return index, true, nil
}

func (c *cache[T]) get(key string) (T, bool, error) {
	var res T
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusFailure)
		return res, false, ErrCacheClosed
	}
	item, found := c.index[key]
	if !found {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusSuccess)
		return res, false, nil
	}
	if !item.expiresAt.IsZero() {
		if item.expiresAt.Compare(time.Now()) < 0 {
			c.mu.RUnlock()
			recordRequest(c.metrics, StatusSuccess)
			return res, false, nil
		}
	}
	c.mu.RUnlock()
	recordRequest(c.metrics, StatusSuccess)
	return item.object, true, nil
}

// Delete an item from the cache. Does nothing if the key is not in the cache.
// It actually sets the item expiration to `now“, so that it will be deleted at
// the cleanup.
func (c *Cache[T]) Delete(object T) error {
	key, err := c.keyFunc(object)
	if err != nil {
		recordRequest(c.metrics, StatusFailure)
		return &CacheError{Reason: ErrInvalidKey, Err: err}
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusFailure)
		return ErrCacheClosed
	}
	if item, ok := c.index[key]; ok {
		// set the item expiration to now
		// so that it will be removed by the janitor
		item.expiresAt = time.Now()
	}
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	return nil
}

// Clear all index from the cache.
// This reallocates the underlying array holding the index,
// so that the memory used by the index is reclaimed.
// A closed cache cannot be cleared.
func (c *cache[T]) Clear() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.index = make(map[string]*item[T])
	c.mu.Unlock()
}

// ListKeys returns a slice of the keys in the cache.
func (c *cache[T]) ListKeys() ([]string, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusFailure)
		return nil, ErrCacheClosed
	}
	keys := make([]string, 0, len(c.index))
	for k := range c.index {
		keys = append(keys, k)
	}
	c.mu.RUnlock()
	recordRequest(c.metrics, StatusSuccess)
	return keys, nil
}

// Resize resizes the cache and returns the number of index removed.
// Size must be greater than zero.
func (c *cache[T]) Resize(size int) (int, error) {
	if size <= 0 {
		recordRequest(c.metrics, StatusFailure)
		return 0, ErrInvalidSize
	}

	c.mu.Lock()
	overflow := len(c.items) - size
	if c.closed {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusFailure)
		return 0, ErrCacheClosed
	}

	// set the new capacity
	c.capacity = size

	if overflow <= 0 {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusSuccess)
		return 0, nil
	}

	if !c.sorted {
		// sort the slice of index by expiration time
		slices.SortFunc(c.items, func(i, j *item[T]) int {
			return i.expiresAt.Compare(j.expiresAt)
		})
		c.sorted = true
	}

	// delete the overflow indexes
	for _, v := range c.items[:overflow] {
		delete(c.index, v.key)
		recordEviction(c.metrics)
		recordDecrement(c.metrics)
	}
	// remove the overflow indexes from the slice
	c.items = c.items[overflow:]
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	return overflow, nil
}

// HasExpired returns true if the item has expired.
func (c *Cache[T]) HasExpired(object T) (bool, error) {
	key, err := c.keyFunc(object)
	if err != nil {
		recordRequest(c.metrics, StatusFailure)
		return false, &CacheError{Reason: ErrInvalidKey, Err: err}
	}

	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusFailure)
		return false, ErrCacheClosed
	}
	item, ok := c.index[key]
	if !ok {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusSuccess)
		return true, nil
	}

	if item.expiresAt.Compare(time.Now()) < 0 {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusSuccess)
		return true, nil
	}

	c.mu.RUnlock()
	recordRequest(c.metrics, StatusSuccess)
	return false, nil
}

// SetExpiration sets the expiration for the given key.
func (c *Cache[T]) SetExpiration(object T, expiration time.Time) error {
	key, err := c.keyFunc(object)
	if err != nil {
		recordRequest(c.metrics, StatusFailure)
		return &CacheError{Reason: ErrInvalidKey, Err: err}
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusFailure)
		return ErrCacheClosed
	}
	item, ok := c.index[key]
	if !ok {
		c.mu.Unlock()
		recordRequest(c.metrics, StatusFailure)
		return ErrNotFound
	}
	item.expiresAt = expiration
	// mark the items as not sorted
	c.sorted = false
	c.mu.Unlock()
	recordRequest(c.metrics, StatusSuccess)
	return nil
}

// GetExpiration returns the expiration for the given key.
// Returns zero if the key is not in the cache or the item
// has already expired.
func (c *Cache[T]) GetExpiration(object T) (time.Time, error) {
	key, err := c.keyFunc(object)
	if err != nil {
		recordRequest(c.metrics, StatusFailure)
		return time.Time{}, &CacheError{Reason: ErrInvalidKey, Err: err}
	}
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusFailure)
		return time.Time{}, ErrCacheClosed
	}
	item, ok := c.index[key]
	if !ok {
		c.mu.RUnlock()
		recordRequest(c.metrics, StatusSuccess)
		return time.Time{}, ErrNotFound
	}
	if !item.expiresAt.IsZero() {
		if item.expiresAt.Compare(time.Now()) < 0 {
			c.mu.RUnlock()
			recordRequest(c.metrics, StatusSuccess)
			return time.Time{}, nil
		}
	}
	c.mu.RUnlock()
	recordRequest(c.metrics, StatusSuccess)
	return item.expiresAt, nil
}

// deleteExpired deletes all expired index from the cache.
// It is called by the janitor.
func (c *cache[T]) deleteExpired() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}

	if !c.sorted {
		// sort the slice of index by expiration time
		slices.SortFunc(c.items, func(i, j *item[T]) int {
			return i.expiresAt.Compare(j.expiresAt)
		})
		c.sorted = true
	}

	t := time.Now()
	index := sort.Search(len(c.items), func(i int) bool {
		// smallest index with an expiration greater than t
		return c.items[i].expiresAt.Compare(t) > 0
	})

	// delete the expired indexes
	for _, v := range c.items[:index] {
		delete(c.index, v.key)
		recordEviction(c.metrics)
		recordDecrement(c.metrics)
	}
	// remove the expired indexes from the slice
	c.items = c.items[index:]
	c.mu.Unlock()
}

type janitor[T any] struct {
	interval time.Duration
	stop     chan bool
}

func (j *janitor[T]) run(c *cache[T]) {
	ticker := time.NewTicker(j.interval)
	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

// buffer is a helper type used to write data to a byte slice
type buffer []byte

// clear clears the buffer
func (s *buffer) clear() {
	*s = (*s)[:0]
}

// writeByteSlice writes a byte slice to the buffer
func (s *buffer) writeByteSlice(v []byte) {
	*s = append(*s, v...)
}

// writeUint64 writes a uint64 to the buffer
// it is written in little endian format
func (s *buffer) writeUint64(v uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	*s = append(*s, buf[:]...)
}

// writeBuf writes the buffer to the file
func (c *cache[T]) writeBuf(file *os.File) error {
	if _, err := file.Write(c.buf); err != nil {
		return err
	}
	// sync the file to disk straight away
	file.Sync()
	return nil
}

// Persist writes the cache to disk
// The cache is written to a temporary file first
// and then renamed to the final file name to atomically
// update the cache file. This is done to avoid corrupting
// the cache file in case of a crash while writing to the file. If a file
// with the same name exists, it is overwritten.
// The cache file is written in the following format:
// key length, key, expiration, data length, data // repeat for each item
// The key length and data length are written as uint64 in little endian format
// The expiration is written as a unix timestamp in seconds as uint64 in little endian format
// The key is written as a byte slice
// The data is written as a json encoded byte slice
func (c *cache[T]) Persist() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.writeToBuf(); err != nil {
		return err
	}

	// create new temp file
	newFile, err := os.Create(fmt.Sprintf("%s.tmp", c.snapshotPath))
	if err != nil {
		errf := os.Remove(fmt.Sprintf("%s.tmp", c.snapshotPath))
		return errors.Join(err, errf)
	}

	if err := c.writeBuf(newFile); err != nil {
		errf := os.Remove(fmt.Sprintf("%s.tmp", c.snapshotPath))
		return errors.Join(err, errf)
	}

	// close the file
	if err := newFile.Close(); err != nil {
		errf := os.Remove(fmt.Sprintf("%s.tmp", c.snapshotPath))
		return errors.Join(err, errf)
	}

	if err := os.Rename(fmt.Sprintf("%s.tmp", c.snapshotPath), c.snapshotPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// writeToBuf writes the cache to the buffer
// no locks are taken, the caller should ensure that
// the cache is not being modified while this function is called.
func (c *cache[T]) writeToBuf() error {
	c.buf.clear()
	for _, item := range c.items {
		data, err := json.Marshal(item.object)
		if err != nil {
			return err
		}

		// write the key, expiration and data to the buffer
		// format: key length, key, expiration, data length, data
		// doing this this way, gives us the ability to read the file
		// without having to read the entire file into memory. This is
		// done for possible future use cases e.g. where the cache file
		// could be very large or for range queries.
		c.buf.writeUint64(uint64(len(item.key)))
		c.buf.writeByteSlice([]byte(item.key))
		// we write the expiration time in nanoseconds as uint64
		// instead of using item.expiresAt.MarshalBinary() because we are only
		// interested in the nano second precision Unix time,
		// everything else can be discarded.
		c.buf.writeUint64(uint64(item.expiresAt.UnixNano()))
		c.buf.writeUint64(uint64(len(data)))
		c.buf.writeByteSlice(data)
	}
	return nil
}

// load reads the cache from disk
// The cache file is read in the following format:
// key length, key, expiration, data length, data // repeat for each item
// This function cannot be called concurrently, and should be called
// before the cache is used.
func (c *cache[T]) load() error {
	file, err := os.Open(c.snapshotPath)
	if err != nil {
		return err
	}
	defer file.Close()

	rd := bufio.NewReader(file)
	items, err := c.readFrom(rd)
	if err != nil {
		return err
	}

	for _, item := range items {
		if len(c.items) >= c.capacity {
			break
		}
		c.items = append(c.items, item)
		c.index[item.key] = item
	}

	if len(c.items) > 0 {
		c.metrics.setCachedItems(float64(len(c.items)))
		c.sorted = false
	}
	return nil
}

func (c *cache[T]) readFrom(rd io.Reader) ([]*item[T], error) {
	items := make([]*item[T], 0)
	for {
		// read until EOF
		item, err := c.readItem(rd)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *cache[T]) readItem(rd io.Reader) (*item[T], error) {
	var (
		buf  = make([]byte, 8)
		item item[T]
	)
	if _, err := io.ReadFull(rd, buf); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, err
	}
	keyLen := binary.LittleEndian.Uint64(buf)
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(rd, key); err != nil {
		return nil, err
	}
	item.key = string(key)

	if _, err := io.ReadFull(rd, buf); err != nil {
		return nil, err
	}
	item.expiresAt = time.Unix(int64(binary.LittleEndian.Uint64(buf)), 0)

	if _, err := io.ReadFull(rd, buf); err != nil {
		return nil, err
	}
	dataLen := binary.LittleEndian.Uint64(buf)
	data := make([]byte, dataLen)
	if _, err := io.ReadFull(rd, data); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &item.object); err != nil {
		return nil, err
	}

	return &item, nil
}
