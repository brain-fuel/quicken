package taskboard

import (
	"reflect"
	"testing"

	"pgregory.net/rapid"
)

func TestModelCodecRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		model := InitialModel()
		model.Query = rapid.StringMatching(`[a-zA-Z0-9 ]{0,32}`).Draw(t, "query")
		model.IncidentDraft = rapid.StringMatching(`[a-zA-Z0-9 ]{0,32}`).Draw(t, "incident")
		model.Online = rapid.Bool().Draw(t, "online")

		codec := ModelCodec()
		data, err := codec.Encode(model)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		decoded, err := codec.Decode(data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !reflect.DeepEqual(decoded, model) {
			t.Fatalf("round trip changed model:\nwant %#v\ngot  %#v", model, decoded)
		}
	})
}

func TestMessageCodecRoundTrip(t *testing.T) {
	messages := []Msg{
		IncidentDraftChanged("network outage"),
		SummaryDraftChanged("packet loss"),
		LocationDraftChanged("us-west"),
		OwnerDraftChanged("Maya"),
		IncidentSubmitted(),
		IncidentSelected(42),
		SeverityChanged(42, SeverityCritical),
		StatusAdvanced(42),
		TaskDraftChanged("drain traffic"),
		TaskAdded(),
		TaskToggled(7),
		NoteDraftChanged("mitigation started"),
		NoteAdded(),
		SearchChanged("outage"),
		FilterRequested(FilterCritical()),
		ConnectivityChanged(false),
		SyncRequested(),
		SaveSucceeded(InitialModel().Incidents),
		SaveFailed("network unavailable"),
		KeepLocalRequested(),
		AcceptRemoteRequested(),
	}
	codec := MessageCodec()
	for _, message := range messages {
		data, err := codec.Encode(message)
		if err != nil {
			t.Fatalf("encode %T: %v", message, err)
		}
		decoded, err := codec.Decode(data)
		if err != nil {
			t.Fatalf("decode %T: %v", message, err)
		}
		if !reflect.DeepEqual(message, decoded) {
			t.Fatalf("message round trip changed %T: %#v", message, decoded)
		}
	}
}

func TestVisibleIncidentsObeysSearchAndFilter(t *testing.T) {
	model := InitialModel()
	model.Query = "checkout"
	model.Filter = FilterCritical()
	visible := VisibleIncidents(model)
	if len(visible) != 1 || visible[0].Title != "Checkout API latency" {
		t.Fatalf("critical checkout query = %#v", visible)
	}

	model.Query = ""
	model.Filter = FilterResolved()
	if visible := VisibleIncidents(model); len(visible) != 0 {
		t.Fatalf("resolved filter unexpectedly returned %#v", visible)
	}
}

func TestStatusProgressionFormsCycle(t *testing.T) {
	status := StatusOpen
	for range 4 {
		status = nextStatus(status)
	}
	if status != StatusOpen {
		t.Fatalf("four transitions from open = %q", status)
	}
}

func TestOfflineMutationQueuesSynchronization(t *testing.T) {
	model := InitialModel()
	model.Online = false
	model.IncidentDraft = "Generator failure"

	step := Update(model, IncidentSubmitted(), LocalSave)
	if !isSyncPending(step.Model.Sync) {
		t.Fatalf("offline mutation sync state = %#v", step.Model.Sync)
	}
	if len(step.Model.Incidents) != len(model.Incidents)+1 {
		t.Fatalf("incident count = %d", len(step.Model.Incidents))
	}
	if step.Model.MutationRevision != model.MutationRevision+1 {
		t.Fatalf("mutation revision = %d", step.Model.MutationRevision)
	}
}

func TestConsecutiveMutationsHaveDistinctRevisions(t *testing.T) {
	model := InitialModel()
	first := Update(model, TaskToggled(1), LocalSave)
	second := Update(first.Model, StatusAdvanced(1), LocalSave)
	if first.Model.MutationRevision == second.Model.MutationRevision {
		t.Fatalf("consecutive mutations reused revision %d", first.Model.MutationRevision)
	}
}

func TestConflictResolutionSelectsRequestedVersion(t *testing.T) {
	model := InitialModel()
	local := model.Incidents[0]
	remote := local
	local.Title = "Local title"
	remote.Title = "Remote title"
	model.Sync = SyncConflicted(Conflict{IncidentID: local.ID, Local: local, Remote: remote})

	localStep := Update(model, KeepLocalRequested(), LocalSave)
	localSelected, _ := selectedIncident(localStep.Model)
	if localSelected.Title != local.Title {
		t.Fatalf("keep local selected %q", localSelected.Title)
	}

	remoteStep := Update(model, AcceptRemoteRequested(), LocalSave)
	remoteSelected, _ := selectedIncident(remoteStep.Model)
	if remoteSelected.Title != remote.Title {
		t.Fatalf("accept remote selected %q", remoteSelected.Title)
	}
}

func isSyncPending(sync SyncState) bool {
	return SyncStateFold(sync, SyncStateCases[bool]{
		SyncIdle: func() bool { return false },
		SyncPending: func() bool { return true },
		SyncRunning: func() bool { return false },
		SyncConflicted: func(Conflict) bool { return false },
	})
}
