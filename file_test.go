package fastcache

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

func TestSaveLoadFile(t *testing.T) {
	var s Stats
	const filePath = "TestSaveLoadFile.fastcache"
	defer os.Remove(filePath)

	const itemsCount = 10000
	const maxBytes = bucketsCount * chunkSize * 2
	c := New(maxBytes)
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		c.Set(k, v)
	}
	c.SaveToFile(filePath)
	s = Stats{}
	c.UpdateStats(&s)
	if s.EntriesCount != itemsCount {
		t.Fatalf("unexpected entriesCount; got %d; want %d", s.EntriesCount, itemsCount)
	}
	c.Reset()

	// Verify LoadFromFile
	c, err := LoadFromFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	s = Stats{}
	c.UpdateStats(&s)
	if s.EntriesCount != itemsCount {
		t.Fatalf("unexpected entriesCount; got %d; want %d", s.EntriesCount, itemsCount)
	}
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		vv := c.Get(nil, k)
		if string(v) != string(vv) {
			t.Fatalf("unexpected cache value for k=%q; got %q; want %q; bucket[0]=%#v", k, vv, v, &c.buckets[0])
		}
	}
	c.Reset()

	// Verify LoadFromFileOrNew
	c = LoadFromFileOrNew(filePath, maxBytes)
	s = Stats{}
	c.UpdateStats(&s)
	if s.EntriesCount != itemsCount {
		t.Fatalf("unexpected entriesCount; got %d; want %d", s.EntriesCount, itemsCount)
	}
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		vv := c.Get(nil, k)
		if string(v) != string(vv) {
			t.Fatalf("unexpected cache value for k=%q; got %q; want %q; bucket[0]=%#v", k, vv, v, &c.buckets[0])
		}
	}
	c.Reset()

	// Verify incorrect maxBytes passed to LoadFromFileOrNew
	c = LoadFromFileOrNew(filePath, maxBytes*10)
	s = Stats{}
	c.UpdateStats(&s)
	if s.EntriesCount != 0 {
		t.Fatalf("unexpected non-zero entriesCount; got %d", s.EntriesCount)
	}
	c.Reset()
}

func TestSaveLoadConcurrent(t *testing.T) {
	c := New(1024)
	defer c.Reset()
	c.Set([]byte("foo"), []byte("bar"))

	stopCh := make(chan struct{})

	// Start concurrent workers that run Get and Set on c.
	var wgWorkers sync.WaitGroup
	for i := 0; i < 5; i++ {
		wgWorkers.Add(1)
		go func() {
			defer wgWorkers.Done()
			var buf []byte
			j := 0
			for {
				k := []byte(fmt.Sprintf("key %d", j))
				v := []byte(fmt.Sprintf("value %d", j))
				c.Set(k, v)
				buf = c.Get(buf[:0], k)
				if string(buf) != string(v) {
					panic(fmt.Errorf("unexpected value for key %q; got %q; want %q", k, buf, v))
				}
				j++
				select {
				case <-stopCh:
					return
				default:
				}
			}
		}()
	}

	// Start concurrent SaveToFile and LoadFromFile calls.
	var wgSavers sync.WaitGroup
	for i := 0; i < 4; i++ {
		wgSavers.Add(1)
		filePath := fmt.Sprintf("TestSaveToFileConcurrent.%d.fastcache", i)
		go func() {
			defer wgSavers.Done()
			defer os.Remove(filePath)
			for j := 0; j < 3; j++ {
				if err := c.SaveToFile(filePath); err != nil {
					panic(fmt.Errorf("cannot save cache to %q: %s", filePath, err))
				}
				cc, err := LoadFromFile(filePath)
				if err != nil {
					panic(fmt.Errorf("cannot load cache from %q: %s", filePath, err))
				}
				var s Stats
				cc.UpdateStats(&s)
				if s.EntriesCount == 0 {
					panic(fmt.Errorf("unexpected empty cache loaded from %q", filePath))
				}
				cc.Reset()
			}
		}()
	}

	wgSavers.Wait()

	close(stopCh)
	wgWorkers.Wait()
}
