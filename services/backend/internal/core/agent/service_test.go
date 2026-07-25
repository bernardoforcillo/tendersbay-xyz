package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type fakeChatRepo struct {
	sessions map[string]postgres.DBChatSession
	messages map[string][]postgres.DBChatMessage
	nextID   int
}

func newFakeChatRepo() *fakeChatRepo {
	return &fakeChatRepo{
		sessions: map[string]postgres.DBChatSession{},
		messages: map[string][]postgres.DBChatMessage{},
	}
}

func (f *fakeChatRepo) CreateSession(_ context.Context, memberID, workspaceID, workbenchID, agentType, title string) (postgres.DBChatSession, error) {
	f.nextID++
	s := postgres.DBChatSession{
		ID: itoa(f.nextID), MemberID: memberID, WorkspaceID: workspaceID,
		AgentType: agentType, Title: title,
	}
	if workbenchID != "" {
		wb := workbenchID
		s.WorkbenchID = &wb
	}
	f.sessions[s.ID] = s
	return s, nil
}

func (f *fakeChatRepo) FindSessionByID(_ context.Context, id string) (postgres.DBChatSession, error) {
	s, ok := f.sessions[id]
	if !ok {
		return postgres.DBChatSession{}, pg.ErrNoRows
	}
	return s, nil
}

func (f *fakeChatRepo) ListSessionsByWorkspace(_ context.Context, workspaceID string) ([]postgres.DBChatSession, error) {
	var out []postgres.DBChatSession
	for _, s := range f.sessions {
		if s.WorkspaceID == workspaceID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeChatRepo) UpdateSession(_ context.Context, id, title, workbenchID string) (postgres.DBChatSession, error) {
	s, ok := f.sessions[id]
	if !ok {
		return postgres.DBChatSession{}, pg.ErrNoRows
	}
	if title != "" {
		s.Title = title
	}
	if workbenchID != "" {
		wb := workbenchID
		s.WorkbenchID = &wb
	}
	f.sessions[id] = s
	return s, nil
}

func (f *fakeChatRepo) DeleteSession(_ context.Context, id string) error {
	delete(f.sessions, id)
	return nil
}

func (f *fakeChatRepo) InsertMessage(_ context.Context, sessionID, role, content string, choices, metadata, tenders json.RawMessage) (postgres.DBChatMessage, error) {
	f.nextID++
	m := postgres.DBChatMessage{ID: "msg-" + itoa(f.nextID), SessionID: sessionID, Role: role, Content: content}
	if choices != nil {
		m.Choices = &choices
	}
	if metadata != nil {
		m.Metadata = &metadata
	}
	if tenders != nil {
		m.Tenders = &tenders
	}
	f.messages[sessionID] = append(f.messages[sessionID], m)
	return m, nil
}

func (f *fakeChatRepo) ListMessagesBySession(_ context.Context, sessionID string) ([]postgres.DBChatMessage, error) {
	return f.messages[sessionID], nil
}

func (f *fakeChatRepo) FindMessageByID(_ context.Context, id string) (postgres.DBChatMessage, error) {
	for _, msgs := range f.messages {
		for _, m := range msgs {
			if m.ID == id {
				return m, nil
			}
		}
	}
	return postgres.DBChatMessage{}, pg.ErrNoRows
}

func itoa(n int) string {
	digits := "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{digits[n%10]}, b...)
		n /= 10
	}
	return string(b)
}

type fakeMemberRepo struct {
	members map[string]bool // "workspaceID|userID" -> is a member
}

func newFakeMemberRepo() *fakeMemberRepo { return &fakeMemberRepo{members: map[string]bool{}} }

func (f *fakeMemberRepo) allow(workspaceID, userID string) {
	f.members[workspaceID+"|"+userID] = true
}

func (f *fakeMemberRepo) LoadMembership(_ context.Context, workspaceID, userID string) (workspace.Membership, error) {
	if f.members[workspaceID+"|"+userID] {
		return workspace.Membership{}, nil
	}
	return workspace.Membership{}, workspace.ErrNotMember
}

type fakeWorkbenches struct {
	// access["workbenchID|userID"] = true → the user may view that workbench.
	access map[string]bool
	// workspace[workbenchID] = workspaceID, for AccessibleWorkbenchIDs.
	workspace map[string]string
}

func newFakeWorkbenches() *fakeWorkbenches {
	return &fakeWorkbenches{access: map[string]bool{}, workspace: map[string]string{}}
}

func (f *fakeWorkbenches) allow(workbenchID, workspaceID, userID string) {
	f.access[workbenchID+"|"+userID] = true
	f.workspace[workbenchID] = workspaceID
}

func (f *fakeWorkbenches) CreateWorkbench(context.Context, string, string, string, string, workbench.Visibility) (workbench.Workbench, error) {
	return workbench.Workbench{}, nil
}

func (f *fakeWorkbenches) CanAccessWorkbench(_ context.Context, userID, workbenchID string) error {
	if f.access[workbenchID+"|"+userID] {
		return nil
	}
	return workbench.ErrWorkbenchNotFound
}

func (f *fakeWorkbenches) AccessibleWorkbenchIDs(_ context.Context, userID, workspaceID string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	for wb, ws := range f.workspace {
		if ws == workspaceID && f.access[wb+"|"+userID] {
			out[wb] = struct{}{}
		}
	}
	return out, nil
}

type fakeTenderSearcher struct{}

func (fakeTenderSearcher) Search(context.Context, tender.SearchParams) (tender.SearchOutput, error) {
	return tender.SearchOutput{}, nil
}

type fakeProfileSource struct {
	profile clientprofile.Profile
	err     error
}

func (f fakeProfileSource) Get(context.Context, string, string) (clientprofile.Profile, error) {
	return f.profile, f.err
}

func newTestService(chatRepo *fakeChatRepo, members *fakeMemberRepo, workbenches Workbenches) *Service {
	registry := NewRegistry("")
	return NewService(registry, chatRepo, credits.NewService(nil, nil, nil), members, workbenches, fakeTenderSearcher{}, fakeProfileSource{err: clientprofile.ErrProfileNotFound})
}

func TestListChats_RejectsNonMember(t *testing.T) {
	chatRepo := newFakeChatRepo()
	members := newFakeMemberRepo()
	svc := newTestService(chatRepo, members, newFakeWorkbenches())

	_, err := svc.ListChats(context.Background(), "user-1", "workspace-1")
	if !errors.Is(err, workspace.ErrNotMember) {
		t.Fatalf("err = %v, want workspace.ErrNotMember", err)
	}
}

func TestListChats_AllowsMember(t *testing.T) {
	chatRepo := newFakeChatRepo()
	members := newFakeMemberRepo()
	members.allow("workspace-1", "user-1")
	svc := newTestService(chatRepo, members, newFakeWorkbenches())

	if _, err := chatRepo.CreateSession(context.Background(), "user-1", "workspace-1", "", "base-chat", "Test"); err != nil {
		t.Fatalf("seed CreateSession: %v", err)
	}

	sessions, err := svc.ListChats(context.Background(), "user-1", "workspace-1")
	if err != nil {
		t.Fatalf("ListChats: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(sessions))
	}
}

func TestCreateChat_RejectsNonMember(t *testing.T) {
	svc := newTestService(newFakeChatRepo(), newFakeMemberRepo(), newFakeWorkbenches())
	_, err := svc.CreateChat(context.Background(), "user-1", "workspace-1", "", "base-chat", "Test")
	if !errors.Is(err, workspace.ErrNotMember) {
		t.Fatalf("err = %v, want workspace.ErrNotMember", err)
	}
}

func TestGetChat_RejectsNonMemberOfChatsWorkspace(t *testing.T) {
	chatRepo := newFakeChatRepo()
	members := newFakeMemberRepo()
	members.allow("workspace-1", "owner")
	svc := newTestService(chatRepo, members, newFakeWorkbenches())

	session, err := chatRepo.CreateSession(context.Background(), "owner", "workspace-1", "", "base-chat", "Test")
	if err != nil {
		t.Fatalf("seed CreateSession: %v", err)
	}

	// "intruder" is not a member of workspace-1 at all.
	if _, err := svc.GetChat(context.Background(), "intruder", session.ID); !errors.Is(err, workspace.ErrNotMember) {
		t.Fatalf("err = %v, want workspace.ErrNotMember", err)
	}

	// Any OTHER member of the same workspace can see it (shared-within-workspace model).
	members.allow("workspace-1", "teammate")
	if _, err := svc.GetChat(context.Background(), "teammate", session.ID); err != nil {
		t.Fatalf("GetChat as teammate: %v", err)
	}
}

func TestGetChat_UnknownChatReturnsNoRows(t *testing.T) {
	svc := newTestService(newFakeChatRepo(), newFakeMemberRepo(), newFakeWorkbenches())
	if _, err := svc.GetChat(context.Background(), "user-1", "does-not-exist"); !errors.Is(err, pg.ErrNoRows) {
		t.Fatalf("err = %v, want pg.ErrNoRows", err)
	}
}

// A chat bound to a workbench is hidden from a workspace member who can't access
// that workbench — workspace membership alone is no longer enough.
func TestGetChat_WorkbenchScoped_HiddenFromNonWorkbenchMember(t *testing.T) {
	chatRepo := newFakeChatRepo()
	members := newFakeMemberRepo()
	members.allow("workspace-1", "teammate") // workspace member, not a workbench member
	wbs := newFakeWorkbenches()
	svc := newTestService(chatRepo, members, wbs)

	session, err := chatRepo.CreateSession(context.Background(), "owner", "workspace-1", "wb-1", "base-chat", "WB chat")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := svc.GetChat(context.Background(), "teammate", session.ID); !errors.Is(err, workbench.ErrWorkbenchNotFound) {
		t.Fatalf("err = %v, want workbench.ErrWorkbenchNotFound", err)
	}
}

func TestGetChat_WorkbenchScoped_VisibleToWorkbenchMember(t *testing.T) {
	chatRepo := newFakeChatRepo()
	wbs := newFakeWorkbenches()
	wbs.allow("wb-1", "workspace-1", "wb-member")
	svc := newTestService(chatRepo, newFakeMemberRepo(), wbs)

	session, _ := chatRepo.CreateSession(context.Background(), "owner", "workspace-1", "wb-1", "base-chat", "WB chat")
	if _, err := svc.GetChat(context.Background(), "wb-member", session.ID); err != nil {
		t.Fatalf("GetChat as workbench member: %v", err)
	}
}

// CreateChat pinned to a workbench requires access to that workbench.
func TestCreateChat_WorkbenchScoped_RejectsNoAccess(t *testing.T) {
	members := newFakeMemberRepo()
	members.allow("workspace-1", "user-1") // workspace member, no workbench access
	svc := newTestService(newFakeChatRepo(), members, newFakeWorkbenches())

	if _, err := svc.CreateChat(context.Background(), "user-1", "workspace-1", "wb-1", "base-chat", "x"); !errors.Is(err, workbench.ErrWorkbenchNotFound) {
		t.Fatalf("err = %v, want workbench.ErrWorkbenchNotFound", err)
	}
}

// ListChats returns every workspace-level chat but only the workbench-scoped
// chats whose workbench the caller can access.
func TestListChats_FiltersWorkbenchChatsByAccess(t *testing.T) {
	chatRepo := newFakeChatRepo()
	members := newFakeMemberRepo()
	members.allow("workspace-1", "teammate")
	wbs := newFakeWorkbenches()
	svc := newTestService(chatRepo, members, wbs)

	if _, err := chatRepo.CreateSession(context.Background(), "owner", "workspace-1", "", "base-chat", "WS chat"); err != nil {
		t.Fatalf("seed ws chat: %v", err)
	}
	if _, err := chatRepo.CreateSession(context.Background(), "owner", "workspace-1", "wb-1", "base-chat", "WB chat"); err != nil {
		t.Fatalf("seed wb chat: %v", err)
	}

	got, err := svc.ListChats(context.Background(), "teammate", "workspace-1")
	if err != nil {
		t.Fatalf("ListChats: %v", err)
	}
	if len(got) != 1 || got[0].WorkbenchID != nil {
		t.Fatalf("teammate should see only the workspace-level chat, got %+v", got)
	}

	// A member who can access the workbench sees both.
	members.allow("workspace-1", "wb-member")
	wbs.allow("wb-1", "workspace-1", "wb-member")
	got, err = svc.ListChats(context.Background(), "wb-member", "workspace-1")
	if err != nil {
		t.Fatalf("ListChats: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("wb-member should see both chats, got %d", len(got))
	}
}

func TestDeleteChat_RejectsNonMemberAndEvictsRegistryOnSuccess(t *testing.T) {
	chatRepo := newFakeChatRepo()
	members := newFakeMemberRepo()
	members.allow("workspace-1", "owner")
	svc := newTestService(chatRepo, members, newFakeWorkbenches())

	session, err := chatRepo.CreateSession(context.Background(), "owner", "workspace-1", "", "base-chat", "Test")
	if err != nil {
		t.Fatalf("seed CreateSession: %v", err)
	}

	if err := svc.DeleteChat(context.Background(), "intruder", session.ID); !errors.Is(err, workspace.ErrNotMember) {
		t.Fatalf("err = %v, want workspace.ErrNotMember", err)
	}
	if _, ok := chatRepo.sessions[session.ID]; !ok {
		t.Fatal("session was deleted despite the caller not being a member")
	}

	if err := svc.DeleteChat(context.Background(), "owner", session.ID); err != nil {
		t.Fatalf("DeleteChat as owner: %v", err)
	}
	if _, ok := chatRepo.sessions[session.ID]; ok {
		t.Fatal("session still present after owner deleted it")
	}
}

func TestDBMessagesToProviderMessages(t *testing.T) {
	got := dbMessagesToProviderMessages([]postgres.DBChatMessage{
		{Role: "user", Content: "Hi"},
		{Role: "assistant", Content: "Hello, how can I help?"},
	})
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if string(got[0].Role) != "user" || got[0].Content != "Hi" {
		t.Fatalf("got[0] = %+v, want {Role: user, Content: Hi}", got[0])
	}
	if string(got[1].Role) != "assistant" || got[1].Content != "Hello, how can I help?" {
		t.Fatalf("got[1] = %+v, want {Role: assistant, Content: Hello, how can I help?}", got[1])
	}
}

func TestGetChatForChoice_RejectsAlreadyAnswered(t *testing.T) {
	chatRepo := newFakeChatRepo()
	members := newFakeMemberRepo()
	members.allow("ws-1", "user-1")
	svc := newTestService(chatRepo, members, newFakeWorkbenches())

	session, _ := chatRepo.CreateSession(context.Background(), "user-1", "ws-1", "", "base-chat", "Test")
	prompt, _ := chatRepo.InsertMessage(context.Background(), session.ID, "choice_prompt", "Q?", json.RawMessage(`[{"key":"A","label":"Yes"}]`), nil, nil)
	// Answered: another message follows it.
	if _, err := chatRepo.InsertMessage(context.Background(), session.ID, "choice_response", "A) Yes", nil, nil, nil); err != nil {
		t.Fatalf("seed choice_response: %v", err)
	}

	if _, err := svc.GetChatForChoice(context.Background(), "user-1", prompt.ID); !errors.Is(err, ErrChoiceNotPending) {
		t.Fatalf("GetChatForChoice: err = %v, want ErrChoiceNotPending", err)
	}
}

func TestGetChatForChoice_AllowsStillPending(t *testing.T) {
	chatRepo := newFakeChatRepo()
	members := newFakeMemberRepo()
	members.allow("ws-1", "user-1")
	svc := newTestService(chatRepo, members, newFakeWorkbenches())

	session, _ := chatRepo.CreateSession(context.Background(), "user-1", "ws-1", "", "base-chat", "Test")
	prompt, _ := chatRepo.InsertMessage(context.Background(), session.ID, "choice_prompt", "Q?", json.RawMessage(`[{"key":"A","label":"Yes"}]`), nil, nil)

	got, err := svc.GetChatForChoice(context.Background(), "user-1", prompt.ID)
	if err != nil {
		t.Fatalf("GetChatForChoice: %v", err)
	}
	if got.ID != session.ID {
		t.Fatalf("session = %+v, want ID=%s", got, session.ID)
	}
}

func TestFormatChoiceAnswer_LooksUpLabelByKey(t *testing.T) {
	choices := json.RawMessage(`[{"key":"A","label":"Private"},{"key":"B","label":"Shared"}]`)
	msg := postgres.DBChatMessage{Choices: &choices}

	got, err := formatChoiceAnswer(msg, "B", "")
	if err != nil {
		t.Fatalf("formatChoiceAnswer: %v", err)
	}
	if got != "B) Shared" {
		t.Fatalf("got = %q, want %q", got, "B) Shared")
	}
}

func TestFormatChoiceAnswer_CustomValue(t *testing.T) {
	choices := json.RawMessage(`[{"key":"A","label":"Private"}]`)
	msg := postgres.DBChatMessage{Choices: &choices}

	got, err := formatChoiceAnswer(msg, "custom", "Aziendale")
	if err != nil {
		t.Fatalf("formatChoiceAnswer: %v", err)
	}
	if got != "Aziendale" {
		t.Fatalf("got = %q, want %q", got, "Aziendale")
	}
}

func TestFormatChoiceAnswer_UnknownKeyErrors(t *testing.T) {
	choices := json.RawMessage(`[{"key":"A","label":"Private"}]`)
	msg := postgres.DBChatMessage{Choices: &choices}

	if _, err := formatChoiceAnswer(msg, "Z", ""); err == nil {
		t.Fatal("formatChoiceAnswer: want error for unknown key, got nil")
	}
}

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int32
	}{
		{"empty string still costs one token", "", 1},
		{"short string floors to one token", "hi", 1},
		{"16 chars is 4 tokens at ~4 chars/token", "0123456789abcdef", 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := estimateTokens(c.in); got != c.want {
				t.Fatalf("estimateTokens(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestPersistAndNotifyTenderResults_PersistsAndSendsResults(t *testing.T) {
	chatRepo := newFakeChatRepo()
	svc := newTestService(chatRepo, newFakeMemberRepo(), newFakeWorkbenches())
	session, _ := chatRepo.CreateSession(context.Background(), "user-1", "ws-1", "", "base-chat", "Test")

	value := int64(500000)
	results := []tender.ScoredTender{{
		Tender: tender.Tender{ID: "t-1", Title: "Cestini intelligenti", Country: "IT", CPV: "34928480", Value: &value},
	}}

	var got TenderResults
	sendTenderResults := func(tr TenderResults) error { got = tr; return nil }

	if err := svc.persistAndNotifyTenderResults(context.Background(), session.ID, results, sendTenderResults); err != nil {
		t.Fatalf("persistAndNotifyTenderResults: %v", err)
	}

	if len(got.Tenders) != 1 || got.Tenders[0].ID != "t-1" {
		t.Fatalf("sendTenderResults got = %+v", got)
	}

	msgs, _ := chatRepo.ListMessagesBySession(context.Background(), session.ID)
	if len(msgs) != 1 || msgs[0].Role != "tender_results" {
		t.Fatalf("persisted messages = %+v, want one tender_results row", msgs)
	}
	if msgs[0].Tenders == nil {
		t.Fatal("persisted tender_results row has no Tenders JSON")
	}
	if !strings.Contains(string(*msgs[0].Tenders), `"id":"t-1"`) {
		t.Fatalf("persisted tenders JSON = %s, want it to contain the tender id", string(*msgs[0].Tenders))
	}
}

func TestBuildProfileContext_OnlyPresentFields(t *testing.T) {
	min := int64(50000)
	ctx := buildProfileContext(clientprofile.Profile{
		Sectors:   []string{"45", "72"},
		Countries: []string{"IT", "DE"},
		ValueMin:  &min,
		Notes:     "Solo appalti verdi",
	})
	for _, want := range []string{"45, 72", "IT, DE", "50000", "Solo appalti verdi"} {
		if !strings.Contains(ctx, want) {
			t.Fatalf("context %q missing %q", ctx, want)
		}
	}
	// Absent fields produce no line.
	if strings.Contains(ctx, "NUTS") || strings.Contains(ctx, "procedura") {
		t.Fatalf("context mentions an absent field: %q", ctx)
	}
}

func TestBuildProfileContext_EmptyProfileIsEmptyString(t *testing.T) {
	if got := buildProfileContext(clientprofile.Profile{}); got != "" {
		t.Fatalf("buildProfileContext(empty) = %q, want empty string", got)
	}
}

func TestPersistAndNotifyTenderResults_PropagatesSendError(t *testing.T) {
	chatRepo := newFakeChatRepo()
	svc := newTestService(chatRepo, newFakeMemberRepo(), newFakeWorkbenches())
	session, _ := chatRepo.CreateSession(context.Background(), "user-1", "ws-1", "", "base-chat", "Test")

	sendErr := errors.New("client disconnected")
	sendTenderResults := func(TenderResults) error { return sendErr }

	results := []tender.ScoredTender{{Tender: tender.Tender{ID: "t-1"}}}
	if err := svc.persistAndNotifyTenderResults(context.Background(), session.ID, results, sendTenderResults); !errors.Is(err, sendErr) {
		t.Fatalf("err = %v, want sendErr", err)
	}
}
