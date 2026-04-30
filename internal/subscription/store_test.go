package subscription

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStored_JSON_RoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 26, 10, 30, 0, 0, time.UTC)
	in := Stored{
		ID:             "s1",
		Name:           "Test",
		URL:            "https://example.test/sub",
		UserAgent:      "ITG-Ray/0.1",
		UpdateInterval: Duration(12 * time.Hour),
		LastSyncAt:     now,
		LastStatus:     "OK imported=3",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Stored
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Fatalf("roundtrip mismatch:\n got  %+v\n want %+v", out, in)
	}
}

func TestStored_ToSyncInput_OmitsMetadata(t *testing.T) {
	s := Stored{
		ID:             "s1",
		Name:           "Test",
		URL:            "https://example.test/sub",
		UserAgent:      "UA",
		UpdateInterval: Duration(time.Hour),
		LastSyncAt:     time.Now(),
		LastStatus:     "ERROR: x",
	}
	in := s.ToSyncInput()
	if in.ID != "s1" || in.Name != "Test" || in.URL != "https://example.test/sub" ||
		in.UserAgent != "UA" || in.UpdateInterval != time.Hour {
		t.Fatalf("ToSyncInput dropped/altered config: %+v", in)
	}
	if in.Auth == nil {
		t.Fatal("Auth must default to AuthNone() (non-nil), since subscription.Sync calls it unconditionally")
	}
}

func TestFileStore_LoadMissingFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	fs := FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	got, err := fs.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d entries, want 0", len(got))
	}
}

func TestFileStore_SaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	fs := FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	want := []Stored{
		{ID: "s1", Name: "A", URL: "https://a.test", UpdateInterval: Duration(12 * time.Hour)},
		{ID: "s2", Name: "B", URL: "https://b.test", LastStatus: "OK imported=1"},
	}
	if err := fs.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := fs.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d:\n got  %+v\n want %+v", i, got[i], want[i])
		}
	}
}

func TestFileStore_LoadLegacyFormat_BackwardsCompatible(t *testing.T) {
	// Simulate a subscriptions.json written by the old cmd/itgray-cli/subs.go
	// (private storedSub: id/name/url/user_agent only).
	dir := t.TempDir()
	path := filepath.Join(dir, "subscriptions.json")
	legacy := `{"subs":[{"id":"s1","name":"Old","url":"https://x.test","user_agent":"UA"}]}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got, err := FileStore{Path: path}.Load()
	if err != nil {
		t.Fatalf("Load legacy: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s1" || got[0].Name != "Old" {
		t.Fatalf("legacy mismatch: %+v", got)
	}
	if got[0].UpdateInterval != 0 || !got[0].LastSyncAt.IsZero() || got[0].LastStatus != "" {
		t.Fatalf("new fields should be zero-valued on legacy load: %+v", got[0])
	}
}

func TestFileStore_UpdateMeta_PartialUpdate(t *testing.T) {
	dir := t.TempDir()
	fs := FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	if err := fs.Save([]Stored{
		{ID: "s1", Name: "A", URL: "https://a.test"},
		{ID: "s2", Name: "B", URL: "https://b.test"},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	at := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	if err := fs.UpdateMeta("s2", at, "ok", "imported=1", nil); err != nil {
		t.Fatalf("UpdateMeta: %v", err)
	}
	got, err := fs.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got[0].LastStatus != "" || !got[0].LastSyncAt.IsZero() {
		t.Fatalf("s1 should be untouched, got %+v", got[0])
	}
	if !got[1].LastSyncAt.Equal(at) || got[1].LastStatus != "ok" || got[1].LastMessage != "imported=1" {
		t.Fatalf("s2 not updated: %+v", got[1])
	}
}

func TestFileStore_UpdateMeta_UnknownID_NoOpNoError(t *testing.T) {
	dir := t.TempDir()
	fs := FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	_ = fs.Save([]Stored{{ID: "s1", Name: "A", URL: "https://a.test"}})
	// Unknown ID — driver may race with a user removing a sub. Should not error.
	if err := fs.UpdateMeta("ghost", time.Now(), "ok", "", nil); err != nil {
		t.Fatalf("UpdateMeta unknown id should be no-op, got error %v", err)
	}
}

func TestFileStore_Save_AtomicReplaceUnderConcurrentReaders(t *testing.T) {
	// Hammer Load while Save runs. Each Load must see either the previous
	// or the next state — never a partial write.
	dir := t.TempDir()
	fs := FileStore{Path: filepath.Join(dir, "subscriptions.json")}
	if err := fs.Save([]Stored{{ID: "s1", Name: "A", URL: "https://a.test"}}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = fs.Save([]Stored{{ID: "s1", Name: "A", URL: "https://a.test", LastStatus: "iter"}})
		}
	}()
	for i := 0; i < 200; i++ {
		got, err := fs.Load()
		if err != nil {
			t.Fatalf("concurrent Load: %v", err)
		}
		if len(got) != 1 || got[0].ID != "s1" {
			t.Fatalf("partial state: %+v", got)
		}
	}
	close(stop)
	wg.Wait()
}

func TestStored_RoundTrip_NewMetadataFields(t *testing.T) {
	dir := t.TempDir()
	fs := FileStore{Path: filepath.Join(dir, "subs.json")}

	expire := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	in := []Stored{{
		ID:          "s1",
		Name:        "A",
		URL:         "https://a.test",
		LastMessage: "imported=3 invalid=0 skipped=0",
		Upload:      111,
		Download:    222,
		Total:       1024 * 1024 * 1024,
		Expire:      &expire,
	}}
	if err := fs.Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := fs.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d, want 1", len(got))
	}
	if got[0].LastMessage != "imported=3 invalid=0 skipped=0" {
		t.Errorf("LastMessage=%q", got[0].LastMessage)
	}
	if got[0].Upload != 111 || got[0].Download != 222 || got[0].Total != 1024*1024*1024 {
		t.Errorf("quota fields lost: %+v", got[0])
	}
	if got[0].Expire == nil || !got[0].Expire.Equal(expire) {
		t.Errorf("Expire round-trip lost: %v", got[0].Expire)
	}
}

func TestFileStore_UpdateMeta_WritesMessageAndUserinfo(t *testing.T) {
	dir := t.TempDir()
	fs := FileStore{Path: filepath.Join(dir, "subs.json")}
	require.NoError(t, fs.Save([]Stored{{ID: "s1", Name: "A", URL: "https://a.test"}}))

	expire := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	ui := &Userinfo{
		Upload: 10, HasUpload: true,
		Download: 20, HasDownload: true,
		Total: 30, HasTotal: true,
		Expire: &expire,
	}
	at := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	require.NoError(t, fs.UpdateMeta("s1", at, "ok", "imported=3", ui))

	got, err := fs.Load()
	require.NoError(t, err)
	require.Equal(t, "ok", got[0].LastStatus)
	require.Equal(t, "imported=3", got[0].LastMessage)
	require.EqualValues(t, 10, got[0].Upload)
	require.EqualValues(t, 20, got[0].Download)
	require.EqualValues(t, 30, got[0].Total)
	require.NotNil(t, got[0].Expire)
	require.True(t, got[0].Expire.Equal(expire))
}

func TestFileStore_UpdateMeta_PartialUserinfo_PreservesUnsetFields(t *testing.T) {
	dir := t.TempDir()
	fs := FileStore{Path: filepath.Join(dir, "subs.json")}
	expire := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, fs.Save([]Stored{{
		ID: "s1", Name: "A", URL: "https://a.test",
		Upload: 999, Download: 888, Total: 1000, Expire: &expire,
	}}))

	// Header arrives with only Total present; Upload/Download/Expire absent
	// or malformed. Only the present field should overwrite stored state.
	ui := &Userinfo{Total: 2000, HasTotal: true}
	at := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	require.NoError(t, fs.UpdateMeta("s1", at, "ok", "", ui))
	got, err := fs.Load()
	require.NoError(t, err)
	require.EqualValues(t, 999, got[0].Upload, "prior Upload preserved (not in header)")
	require.EqualValues(t, 888, got[0].Download, "prior Download preserved (not in header)")
	require.EqualValues(t, 2000, got[0].Total, "Total updated from header")
	require.NotNil(t, got[0].Expire, "prior Expire preserved (not in header)")
}

func TestFileStore_UpdateMeta_NilUserinfo_PreservesPriorQuota(t *testing.T) {
	dir := t.TempDir()
	fs := FileStore{Path: filepath.Join(dir, "subs.json")}
	expire := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, fs.Save([]Stored{{
		ID: "s1", Name: "A", URL: "https://a.test",
		Upload: 999, Download: 888, Total: 1000, Expire: &expire,
	}}))

	at := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, fs.UpdateMeta("s1", at, "error", "boom", nil))

	got, err := fs.Load()
	require.NoError(t, err)
	require.Equal(t, "error", got[0].LastStatus)
	require.Equal(t, "boom", got[0].LastMessage)
	require.EqualValues(t, 999, got[0].Upload, "prior Upload preserved")
	require.EqualValues(t, 888, got[0].Download, "prior Download preserved")
	require.EqualValues(t, 1000, got[0].Total, "prior Total preserved")
	require.NotNil(t, got[0].Expire)
}
