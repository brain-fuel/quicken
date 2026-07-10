package quicken

import (
	"sync"
	"testing"
)

func TestMemoryStorePutGetDelete(t *testing.T) {
	st := NewMemoryStore()
	sess := &LiveSession{regions: map[string]*regionState{}}
	st.Put("tok", sess)
	got, ok := st.Get("tok")
	if !ok || got != sess {
		t.Fatalf("Get after Put = %v, %v", got, ok)
	}
	st.Delete("tok")
	if _, ok := st.Get("tok"); ok {
		t.Fatal("Get after Delete still present")
	}
}

func TestMemoryStoreGetMissing(t *testing.T) {
	if _, ok := NewMemoryStore().Get("nope"); ok {
		t.Fatal("Get on empty store returned ok")
	}
}

func TestNewTokenUniqueAndHex(t *testing.T) {
	a, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := newToken()
	if a == b {
		t.Fatal("two tokens collided")
	}
	if len(a) != 32 {
		t.Fatalf("token len = %d, want 32", len(a))
	}
	for _, c := range a {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("non-hex char %q in token", c)
		}
	}
}

func TestLiveSessionGetSet(t *testing.T) {
	s := &LiveSession{regions: map[string]*regionState{}}
	if _, ok := s.get("r"); ok {
		t.Fatal("empty session returned a region")
	}
	rs := &regionState{state: 1, lastDynamics: []string{"a"}}
	s.set("r", rs)
	got, ok := s.get("r")
	if !ok || got != rs {
		t.Fatalf("get after set = %v, %v", got, ok)
	}
}

func TestLiveSessionWithRegionMissingReturnsFalse(t *testing.T) {
	s := &LiveSession{regions: map[string]*regionState{}}
	called := false
	found := s.withRegion("nope", func(*regionState) { called = true })
	if found || called {
		t.Fatalf("withRegion on a missing id: found=%v called=%v, want false/false", found, called)
	}
}

func TestLiveSessionWithRegionRunsAndCommits(t *testing.T) {
	s := &LiveSession{regions: map[string]*regionState{
		"r": {state: 1, lastDynamics: []string{"a"}},
	}}
	found := s.withRegion("r", func(rs *regionState) {
		rs.state = rs.state.(int) + 1
	})
	if !found {
		t.Fatal("withRegion on a present id returned false")
	}
	rs, _ := s.get("r")
	if rs.state.(int) != 2 {
		t.Fatalf("state after withRegion = %v, want 2", rs.state)
	}
}

// TestLiveSessionWithRegionSerializesConcurrentReadModifyWrite drives two
// goroutines concurrently against the SAME regionState through withRegion,
// each doing a non-atomic read-modify-write (read state, add 1, write state
// back). Without withRegion holding the session lock across the whole
// operation, two goroutines racing here would both drop increments and the
// final count would fall short of the total; with the lock held for the
// full closure, every increment is serialized and none are lost. Run with
// -race to also confirm no data race is reported on the regionState fields.
func TestLiveSessionWithRegionSerializesConcurrentReadModifyWrite(t *testing.T) {
	s := &LiveSession{regions: map[string]*regionState{
		"c": {state: 0, lastDynamics: nil},
	}}

	const goroutines = 8
	const incsPerGoroutine = 500
	want := goroutines * incsPerGoroutine

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < incsPerGoroutine; i++ {
				s.withRegion("c", func(rs *regionState) {
					rs.state = rs.state.(int) + 1
				})
			}
		}()
	}
	wg.Wait()

	rs, ok := s.get("c")
	if !ok {
		t.Fatal("region missing after concurrent withRegion calls")
	}
	if rs.state.(int) != want {
		t.Fatalf("final state = %d, want %d (a lower count means the read-modify-write raced)", rs.state.(int), want)
	}
}
