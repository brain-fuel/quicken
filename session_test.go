package quicken

import "testing"

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
