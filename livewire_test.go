package quicken

import (
	"encoding/json"
	"testing"
)

func TestDecodeClientEvent(t *testing.T) {
	m, err := decodeClient([]byte(`{"type":"event","region":"c","event":"inc","payload":{"by":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != "event" || m.Region != "c" || m.Event != "inc" {
		t.Fatalf("decoded = %+v", m)
	}
	if m.Payload["by"] != float64(2) {
		t.Fatalf("payload by = %v", m.Payload["by"])
	}
}

func TestDecodeClientResume(t *testing.T) {
	m, err := decodeClient([]byte(`{"type":"resume","token":"abc"}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != "resume" || m.Token != "abc" {
		t.Fatalf("decoded = %+v", m)
	}
}

func TestDecodeClientRejectsGarbage(t *testing.T) {
	if _, err := decodeClient([]byte(`{not json`)); err == nil {
		t.Fatal("expected error on bad json")
	}
}

func TestFirstMsgCarriesStaticsAndDynamics(t *testing.T) {
	tr := Slots([]string{"<b>", "</b>"}, []string{"7"})
	m := firstMsg("c", tr)
	if m.Type != "first" || m.Region != "c" {
		t.Fatalf("msg = %+v", m)
	}
	if len(m.Statics) != 2 || m.Statics[0] != "<b>" || m.Dynamics[0] != "7" {
		t.Fatalf("statics/dynamics = %v %v", m.Statics, m.Dynamics)
	}
}

func TestFullMsgCarriesStaticsAndDynamics(t *testing.T) {
	tr := Slots([]string{"<b>", "</b>"}, []string{"7"})
	m := fullMsg("c", tr)
	if m.Type != "full" || m.Region != "c" {
		t.Fatalf("msg = %+v", m)
	}
	if len(m.Statics) != 2 || m.Statics[0] != "<b>" || m.Dynamics[0] != "7" {
		t.Fatalf("statics/dynamics = %v %v", m.Statics, m.Dynamics)
	}
}

func TestPatchMsgOnlyChangedSlots(t *testing.T) {
	m := patchMsg("c", map[int]string{0: "8"})
	if m.Type != "patch" || m.Changed[0] != "8" {
		t.Fatalf("patch = %+v", m)
	}
}

func TestErrorMsgCarriesMessage(t *testing.T) {
	m := errorMsg("c", "boom")
	if m.Type != "error" || m.Region != "c" || m.Message != "boom" {
		t.Fatalf("errorMsg = %+v", m)
	}
}

func TestEncodeServerRoundTrips(t *testing.T) {
	b, err := encodeServer(errorMsg("c", "boom"))
	if err != nil {
		t.Fatal(err)
	}
	var back serverMsg
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Type != "error" || back.Message != "boom" {
		t.Fatalf("round trip = %+v", back)
	}
}

func TestEncodeServerChangedKeysAreStrings(t *testing.T) {
	// json.Marshal of a map[int]string encodes integer keys as strings; the
	// client must parseInt the key. Verify the wire shape directly.
	b, err := encodeServer(patchMsg("c", map[int]string{0: "8"}))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var changed map[string]string
	if err := json.Unmarshal(raw["changed"], &changed); err != nil {
		t.Fatal(err)
	}
	if changed["0"] != "8" {
		t.Fatalf("changed = %v, want key \"0\"", changed)
	}
}

func TestTreeStaticsAndDynamicsAccessors(t *testing.T) {
	tr := Slots([]string{"<b>", "</b>"}, []string{"7"})
	if s := treeStatics(tr); len(s) != 2 || s[0] != "<b>" || s[1] != "</b>" {
		t.Fatalf("treeStatics = %v", s)
	}
	if d := treeDynamics(tr); len(d) != 1 || d[0] != "7" {
		t.Fatalf("treeDynamics = %v", d)
	}
}

func TestRenderLiveHTMLWrapsDynamics(t *testing.T) {
	tr := Slots([]string{"<p>", "!</p>"}, []string{"hi"})
	got := renderLiveHTML(tr)
	want := `<p><q-d data-qi="0">hi</q-d>!</p>`
	if got != want {
		t.Fatalf("renderLiveHTML = %q, want %q", got, want)
	}
}

func TestRenderLiveHTMLWrapsMultipleDynamics(t *testing.T) {
	tr := Slots([]string{"a", "b", "c"}, []string{"1", "2"})
	got := renderLiveHTML(tr)
	want := `a<q-d data-qi="0">1</q-d>b<q-d data-qi="1">2</q-d>c`
	if got != want {
		t.Fatalf("renderLiveHTML = %q, want %q", got, want)
	}
}

func TestRenderLiveHTMLNoDynamics(t *testing.T) {
	if got := renderLiveHTML(Text("plain")); got != "plain" {
		t.Fatalf("renderLiveHTML(Text) = %q", got)
	}
}

func TestRenderLiveHTMLEmptyTree(t *testing.T) {
	if got := renderLiveHTML(Tree{}); got != "" {
		t.Fatalf("renderLiveHTML(Tree{}) = %q, want empty", got)
	}
}
