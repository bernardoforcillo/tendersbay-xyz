package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	bagent "github.com/buildwithgo/berrygem/agent"
	"github.com/buildwithgo/berrygem/providers"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
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

// fakeTenders satisfies the combined Tenders port. Both halves return empty
// values: no test here drives a tool through berrygem (that needs a live
// provider), so these exist to let the Service be constructed at all.
type fakeTenders struct{}

func (fakeTenders) Search(context.Context, tender.SearchParams) (tender.SearchOutput, error) {
	return tender.SearchOutput{}, nil
}

func (fakeTenders) GetTender(context.Context, tender.GetTenderParams) (tender.TenderDetail, error) {
	return tender.TenderDetail{}, nil
}

type fakeDocuments struct{}

func (fakeDocuments) Excerpts(context.Context, document.ExcerptQuery) (document.ExcerptResult, error) {
	return document.ExcerptResult{}, nil
}

// fakeCompanies satisfies the Companies port with an empty dossier and no-op
// writes. Like fakeTenders and fakeDocuments it exists so the Service can be
// constructed: no test here drives a company tool through berrygem, which would
// need a live provider.
type fakeCompanies struct{}

func (fakeCompanies) GetDossier(context.Context, string, string) (company.Dossier, error) {
	return company.Dossier{}, company.ErrDossierNotFound
}

func (fakeCompanies) RecordFact(context.Context, string, string, company.Fact, company.PromptSource, *int64) error {
	return nil
}

func (fakeCompanies) RecordRequirements(context.Context, string, string, int64, []company.Requirement) ([]company.Requirement, error) {
	return nil, nil
}

func (fakeCompanies) CheckEligibility(context.Context, string, string, int64, string) (company.Assessment, error) {
	return company.Assessment{}, nil
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
	return NewService(registry, chatRepo, credits.NewService(nil, nil, nil), members, workbenches, fakeTenders{}, fakeDocuments{}, fakeCompanies{}, fakeProfileSource{err: clientprofile.ErrProfileNotFound}, "test-pod")
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

func TestDeleteChat_RejectsNonMemberAndDeletesOnSuccess(t *testing.T) {
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

// ── conversationForTurn ─────────────────────────────────────────────────────
//
// These replace TestDBMessagesToProviderMessages, which asserted the verbatim
// {Role, Content} passthrough — i.e. it was the written specification of the
// defect, exercising only the two roles that happened to survive it. Its
// deletion is the honest signal that the semantics changed.

// raw is a one-liner for the *json.RawMessage columns, which the fake repo and
// the real schema both model as pointers.
func raw(s string) *json.RawMessage {
	m := json.RawMessage(s)
	return &m
}

// providerKnownRoles is the set berrygem's OpenAI-compatible provider handles
// explicitly. Anything outside it falls into that provider's default: branch,
// which silently rewrites the message as a user turn — no error, no log. Note
// RoleTool is absent on purpose: it is a role the provider knows, but emitting
// one requires a tool_call_id that was never persisted, and an orphan tool
// message is a hard 400 rather than a silent coercion.
var providerKnownRoles = map[providers.Role]bool{
	providers.RoleSystem:    true,
	providers.RoleUser:      true,
	providers.RoleAssistant: true,
}

// assertProviderSafe checks the invariant the whole mapper exists to hold, and
// it is stronger than "the payloads survive": no message reaching the provider
// may carry a role the provider does not handle, and none may be empty. It is
// asserted as its own pass over the messages — not folded into the per-row
// expectations — so a new role added later fails here even if nobody thought
// to write a case for it. Safe to apply to a provider prompt too, which is the
// mapper's output with the system message already prepended.
func assertProviderSafe(t *testing.T, got []providers.Message) {
	t.Helper()
	for i, m := range got {
		if !providerKnownRoles[m.Role] {
			t.Fatalf("got[%d].Role = %q, which reaches the provider's default: branch and is silently coerced", i, m.Role)
		}
		if m.Content == "" {
			t.Fatalf("got[%d] (%s) has empty content; an empty message is what the model hallucinates around", i, m.Role)
		}
	}
}

// assertMapperOutput is assertProviderSafe plus the one check that applies to
// conversationForTurn's own output and not to a finished prompt:
// ensureSystemMessage prepends the agent's instructions only when the first
// message is not already a system one, so a mapped row landing at index 0 with
// RoleSystem would silently delete the entire system prompt.
func assertMapperOutput(t *testing.T, got []providers.Message) {
	t.Helper()
	assertProviderSafe(t, got)
	if len(got) > 0 && got[0].Role == providers.RoleSystem {
		t.Fatal("got[0].Role = system; that suppresses berrygem's ensureSystemMessage and drops the instructions")
	}
}

func TestConversationForTurn_MapsEveryPersistedRole(t *testing.T) {
	got := conversationForTurn(context.Background(), []postgres.DBChatMessage{
		{Role: string(roleUser), Content: "Cerca bandi per cestini intelligenti"},
		{Role: string(roleTenderResults), Tenders: raw(`[{"id":"TED-123","title":"Cestini"}]`)},
		{Role: string(roleAssistant), Content: "Ho trovato 1 bando."},
		{
			Role:    string(roleChoicePrompt),
			Content: "Vuoi salvarlo in un workbench?",
			Choices: raw(`[{"key":"A","label":"Sì, privato"},{"key":"B","label":"No","description":"lascia stare"}]`),
		},
		{Role: string(roleChoiceResponse), Content: "A) Sì, privato"},
	})

	assertMapperOutput(t, got)

	if len(got) != 5 {
		t.Fatalf("len(got) = %d, want 5 (one per persisted row)", len(got))
	}

	if got[0].Role != providers.RoleUser || got[0].Content != "Cerca bandi per cestini intelligenti" {
		t.Fatalf("user row mapped to %+v", got[0])
	}

	// The tender payload lives in the tenders column; the row's content is ""
	// by construction, so a verbatim copy sends an empty message. Assert on
	// the tender id rather than the exact wording so the test does not calcify
	// the framing text.
	if got[1].Role != providers.RoleUser {
		t.Fatalf("tender_results mapped to role %q, want user", got[1].Role)
	}
	if !strings.Contains(got[1].Content, "TED-123") {
		t.Fatalf("tender_results content = %q, want it to carry the tender id", got[1].Content)
	}

	if got[2].Role != providers.RoleAssistant || got[2].Content != "Ho trovato 1 bando." {
		t.Fatalf("assistant row mapped to %+v", got[2])
	}

	// The choice prompt is the assistant's own question. Verbatim copying sent
	// it as role "choice_prompt", which the provider coerces to user — the
	// assistant's words attributed to the user.
	if got[3].Role != providers.RoleAssistant {
		t.Fatalf("choice_prompt mapped to role %q, want assistant", got[3].Role)
	}
	for _, want := range []string{"Vuoi salvarlo in un workbench?", "A) Sì, privato", "B) No"} {
		if !strings.Contains(got[3].Content, want) {
			t.Fatalf("choice_prompt content %q is missing %q; without the options the next turn's \"A) …\" answer is uninterpretable", got[3].Content, want)
		}
	}

	if got[4].Role != providers.RoleUser || got[4].Content != "A) Sì, privato" {
		t.Fatalf("choice_response mapped to %+v, want a verbatim user message", got[4])
	}
}

func TestConversationForTurn_OnlyTheLatestTenderResultsCarriesItsPayload(t *testing.T) {
	got := conversationForTurn(context.Background(), []postgres.DBChatMessage{
		{Role: string(roleUser), Content: "prima ricerca"},
		{Role: string(roleTenderResults), Tenders: raw(`[{"id":"TED-OLD-1"},{"id":"TED-OLD-2"}]`)},
		{Role: string(roleUser), Content: "seconda ricerca"},
		{Role: string(roleTenderResults), Tenders: raw(`[{"id":"TED-NEW-1"}]`)},
	})

	assertMapperOutput(t, got)
	if len(got) != 4 {
		t.Fatalf("len(got) = %d, want 4", len(got))
	}

	// The older row keeps a trace (so the model knows a search happened) but
	// not its payload — that is what bounds prompt growth across a long
	// session.
	if strings.Contains(got[1].Content, "TED-OLD-1") {
		t.Fatalf("superseded tender_results still carries its payload: %q", got[1].Content)
	}
	if !strings.Contains(got[1].Content, "2") {
		t.Fatalf("superseded placeholder %q should name how many results it held", got[1].Content)
	}
	if !strings.Contains(got[3].Content, "TED-NEW-1") {
		t.Fatalf("latest tender_results content = %q, want the payload", got[3].Content)
	}
}

func TestConversationForTurn_DropsUnknownRoles(t *testing.T) {
	in := []postgres.DBChatMessage{
		{Role: string(roleUser), Content: "domanda"},
		{Role: "criteria_results", Content: "una griglia di criteri"},
		{Role: string(roleAssistant), Content: "risposta"},
	}
	got := conversationForTurn(context.Background(), in)

	assertMapperOutput(t, got)
	if len(got) != len(in)-1 {
		t.Fatalf("len(got) = %d, want %d (the unknown role is dropped, not passed through)", len(got), len(in)-1)
	}
	for i, m := range got {
		if strings.Contains(m.Content, "una griglia di criteri") {
			t.Fatalf("got[%d] carries the unknown row's content; a turn attributed to the wrong speaker is worse than an absent one", i)
		}
	}
}

func TestConversationForTurn_DropsRowsThatMapToNothing(t *testing.T) {
	// A tender_results row with no tenders column never carried a payload, and
	// an assistant row with empty content is a turn that produced nothing.
	// Neither may reach the provider as an empty message, and neither may
	// panic.
	got := conversationForTurn(context.Background(), []postgres.DBChatMessage{
		{Role: string(roleUser), Content: "domanda"},
		{Role: string(roleTenderResults)},
		{Role: string(roleAssistant), Content: ""},
	})

	assertMapperOutput(t, got)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (only the user row maps to content)", len(got))
	}
}

func TestConversationForTurn_ChoicePromptWithoutParsableOptionsKeepsItsQuestion(t *testing.T) {
	got := conversationForTurn(context.Background(), []postgres.DBChatMessage{
		{Role: string(roleChoicePrompt), Content: "Privato o condiviso?"},
		{Role: string(roleChoicePrompt), Content: "E il nome?", Choices: raw(`not json`)},
	})

	assertMapperOutput(t, got)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (a question without its options is degraded, not dropped)", len(got))
	}
	for i, want := range []string{"Privato o condiviso?", "E il nome?"} {
		if got[i].Content != want {
			t.Fatalf("got[%d].Content = %q, want %q", i, got[i].Content, want)
		}
	}
}

func TestConversationForTurn_EmptyHistoryIsEmpty(t *testing.T) {
	if got := conversationForTurn(context.Background(), nil); len(got) != 0 {
		t.Fatalf("conversationForTurn(nil) = %+v, want no messages", got)
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

// ── driving a real turn ─────────────────────────────────────────────────────
//
// Before this change nothing could drive runTurn: Registry.BuildAgent
// hardcodes fireworks.New, so every turn needed a live provider and an API
// key. Service.agentOpts is the seam — an option appended after the
// registry's overrides its provider, because berrygem applies options in order
// and WithProvider plainly assigns. The tests below are the reason the defect
// was invisible for so long: the thing that was wrong was the exact message
// list handed to the provider, and nothing could see it.

// recordingProvider is a providers.Provider that answers with a canned reply
// and keeps a copy of every prompt it was handed. The copy matters — berrygem
// appends to the same backing array as its turn loop proceeds.
type recordingProvider struct {
	mu      sync.Mutex
	prompts [][]providers.Message
	replies []string
	calls   int
	// recvErr, when set, makes the stream fail on its first Recv — the
	// provider blowing up after the request was sent and before a single
	// content chunk came back. berrygem turns that into a value on its error
	// channel, which is one of the two ways a turn reaches runTurn's failure
	// path (the other being a sendToken the client can no longer receive).
	recvErr error
}

func (p *recordingProvider) Chat(context.Context, *providers.ChatRequest) (*providers.ChatResponse, error) {
	return nil, errors.New("recordingProvider: Chat must not be called; the agent path is streaming only")
}

func (p *recordingProvider) Stream(_ context.Context, req *providers.ChatRequest) (providers.StreamIterator, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	prompt := make([]providers.Message, len(req.Messages))
	copy(prompt, req.Messages)
	p.prompts = append(p.prompts, prompt)
	reply := "ok"
	if p.calls < len(p.replies) {
		reply = p.replies[p.calls]
	}
	p.calls++
	if p.recvErr != nil {
		return &cannedStream{recvErr: p.recvErr}, nil
	}
	return &cannedStream{chunks: []providers.StreamChunk{{Content: reply}}}, nil
}

// callCount reports how many times the provider was reached. It answers the
// only question a rejected turn has: whether the request got out at all.
func (p *recordingProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// promptAt returns the message list handed to the provider on the (0-indexed)
// nth call, as role/content pairs rendered for a readable failure message.
func (p *recordingProvider) promptAt(t *testing.T, n int) []providers.Message {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if n >= len(p.prompts) {
		t.Fatalf("provider was called %d time(s), wanted at least %d", len(p.prompts), n+1)
	}
	return p.prompts[n]
}

type cannedStream struct {
	chunks  []providers.StreamChunk
	at      int
	recvErr error
}

func (s *cannedStream) Recv() (*providers.StreamChunk, error) {
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	if s.at >= len(s.chunks) {
		return nil, io.EOF
	}
	c := s.chunks[s.at]
	s.at++
	return &c, nil
}

func (s *cannedStream) Close() error { return nil }

// turnHarness wires a Service to a recordingProvider and swallows every stream
// callback, so a test only has to say what the user sent and assert on what
// the provider saw.
type turnHarness struct {
	svc      *Service
	repo     *fakeChatRepo
	provider *recordingProvider
	session  postgres.DBChatSession
	usageCh  chan credits.Usage
}

func newTurnHarness(t *testing.T, replies ...string) *turnHarness {
	t.Helper()
	repo := newFakeChatRepo()
	members := newFakeMemberRepo()
	members.allow("ws-1", "user-1")
	svc := newTestService(repo, members, newFakeWorkbenches())
	// RegisterDefaults so the agent carries instructions and berrygem's
	// ensureSystemMessage prepends a system message — the assertions below
	// pin its presence and position.
	svc.registry.RegisterDefaults()
	prov := &recordingProvider{replies: replies}
	svc.agentOpts = []bagent.Option{bagent.WithProvider(prov)}

	session, err := repo.CreateSession(context.Background(), "user-1", "ws-1", "", string(AgentTypeBaseChat), "Test")
	if err != nil {
		t.Fatalf("seed CreateSession: %v", err)
	}
	// Buffered generously: sendUsage writes one value per turn and nothing in
	// these tests drains it.
	return &turnHarness{svc: svc, repo: repo, provider: prov, session: session, usageCh: make(chan credits.Usage, 8)}
}

func (h *turnHarness) send(t *testing.T, message string) {
	t.Helper()
	if err := h.sendWith(message, func(string) error { return nil }); err != nil {
		t.Fatalf("ChatStream(%q): %v", message, err)
	}
}

// sendWith runs a turn with a caller-supplied token callback and returns the
// turn's error instead of failing the test — the seam the billing tests need,
// since a sendToken that fails is exactly how a client disconnect surfaces
// (consumeStream returns the callback's error, and with no pending choice
// runTurn classifies it as a genuine failure).
func (h *turnHarness) sendWith(message string, sendToken StreamToken) error {
	return h.svc.ChatStream(context.Background(), h.session.ID, "user-1", "ws-1", message, string(AgentTypeBaseChat),
		sendToken,
		func(ChoicePrompt) error { return nil },
		func(TenderResults) error { return nil },
		func(ToolCall) error { return nil },
		h.usageCh,
	)
}

// usage returns the single credits.Usage the turn reported, failing if none
// was reported at all — which is the pre-fix behaviour of an aborted turn.
func (h *turnHarness) usage(t *testing.T) credits.Usage {
	t.Helper()
	select {
	case u := <-h.usageCh:
		return u
	default:
		t.Fatal("the turn reported no usage; a turn that already reached the provider has already cost money")
		return credits.Usage{}
	}
}

// assertNoMoreUsage guards the other direction: a turn must report usage once,
// not twice. Double-reporting would double-bill, because the handler deducts
// whatever it finds on the channel.
func (h *turnHarness) assertNoMoreUsage(t *testing.T) {
	t.Helper()
	select {
	case u := <-h.usageCh:
		t.Fatalf("the turn reported usage twice; the second value was %+v", u)
	default:
	}
}

// assistantMetadata returns the parsed instrumentation stamp on the nth
// (0-indexed) persisted assistant reply.
func (h *turnHarness) assistantMetadata(t *testing.T, n int) turnMetadata {
	t.Helper()
	msgs, err := h.repo.ListMessagesBySession(context.Background(), h.session.ID)
	if err != nil {
		t.Fatalf("ListMessagesBySession: %v", err)
	}
	seen := 0
	for _, m := range msgs {
		if chatRole(m.Role) != roleAssistant {
			continue
		}
		if seen != n {
			seen++
			continue
		}
		if m.Metadata == nil {
			t.Fatalf("assistant reply %d has a NULL metadata column; the prompt-size measurement was not stamped", n)
		}
		var md turnMetadata
		if err := json.Unmarshal(*m.Metadata, &md); err != nil {
			t.Fatalf("assistant reply %d metadata %s: %v", n, string(*m.Metadata), err)
		}
		return md
	}
	t.Fatalf("session has fewer than %d assistant replies", n+1)
	return turnMetadata{}
}

// assertPrompt compares one recorded prompt against an expected transcript,
// where each want is "role:content". The system message is matched on its role
// alone (its content is the full instructions block).
func assertPrompt(t *testing.T, got []providers.Message, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("prompt has %d message(s), want %d:\n got: %s\nwant: %s", len(got), len(want), renderPrompt(got), strings.Join(want, " | "))
	}
	for i, w := range want {
		role, content, _ := strings.Cut(w, ":")
		if string(got[i].Role) != role {
			t.Fatalf("prompt[%d].Role = %q, want %q\n got: %s\nwant: %s", i, got[i].Role, role, renderPrompt(got), strings.Join(want, " | "))
		}
		if content != "" && got[i].Content != content {
			t.Fatalf("prompt[%d].Content = %q, want %q\n got: %s\nwant: %s", i, got[i].Content, content, renderPrompt(got), strings.Join(want, " | "))
		}
	}
}

func renderPrompt(msgs []providers.Message) string {
	parts := make([]string, len(msgs))
	for i, m := range msgs {
		content := m.Content
		if m.Role == providers.RoleSystem {
			content = "<instructions>"
		}
		parts[i] = string(m.Role) + ":" + content
	}
	return strings.Join(parts, " | ")
}

// TestRunTurn_SecondTurnSeesFirstTurnsReply is the regression pin for the
// whole change. Both of its first two assertions fail on the pre-fix code:
// turn 1 sent [system, u1, u1] because the user row was persisted before the
// history was read and then appended again by chat.SendStream, and turn 2 sent
// [system, u1, u1, u2] because the warm in-memory chat never recorded the
// assistant reply at all (berrygem records it via CompleteStream, which this
// service never called).
func TestRunTurn_SecondTurnSeesFirstTurnsReply(t *testing.T) {
	h := newTurnHarness(t, "a1", "a2", "a3")

	h.send(t, "u1")
	assertPrompt(t, h.provider.promptAt(t, 0), "system:", "user:u1")

	h.send(t, "u2")
	assertPrompt(t, h.provider.promptAt(t, 1), "system:", "user:u1", "assistant:a1", "user:u2")

	h.send(t, "u3")
	assertPrompt(t, h.provider.promptAt(t, 2), "system:", "user:u1", "assistant:a1", "user:u2", "assistant:a2", "user:u3")
}

// TestRunTurn_TenderResultsReachTheModelOnTheNextTurn covers the path the
// eForms tools are being built on: a tool result that was persisted for the UI
// must also be in the model's context next turn, or multi-step reasoning over
// it is structurally impossible.
func TestRunTurn_TenderResultsReachTheModelOnTheNextTurn(t *testing.T) {
	h := newTurnHarness(t, "Ho trovato un bando.", "Il secondo è …")

	h.send(t, "cerca cestini intelligenti")
	value := int64(500000)
	if err := h.svc.persistAndNotifyTenderResults(context.Background(), h.session.ID,
		[]tender.ScoredTender{{Tender: tender.Tender{ID: "TED-123", Title: "Cestini"}}, {Tender: tender.Tender{ID: "TED-456", Title: "Raccolta", Value: &value}}},
		func(TenderResults) error { return nil },
	); err != nil {
		t.Fatalf("persistAndNotifyTenderResults: %v", err)
	}

	h.send(t, "il secondo mi interessa")

	got := h.provider.promptAt(t, 1)
	assertProviderSafe(t, got)
	var carried bool
	for _, m := range got {
		if strings.Contains(m.Content, "TED-456") {
			carried = true
		}
	}
	if !carried {
		t.Fatalf("no message carries the tender payload the user was shown:\n%s", renderPrompt(got))
	}
}

// TestSubmitChoice_ResumesWithTheQuestionAsAssistant is the cold-resume
// scenario — previously reachable only by routing the follow-up RPC to a
// second pod, now a plain unit test because there is no warm pod to be cold
// against. The question must come back as the assistant's, immediately
// followed by the answer as the user's; verbatim copying inverted the speaker
// on the first of those two.
func TestSubmitChoice_ResumesWithTheQuestionAsAssistant(t *testing.T) {
	h := newTurnHarness(t, "Creato.")

	prompt, err := h.repo.InsertMessage(context.Background(), h.session.ID, string(roleChoicePrompt),
		"Privato o condiviso?", json.RawMessage(`[{"key":"A","label":"Privato"},{"key":"B","label":"Condiviso"}]`), nil, nil)
	if err != nil {
		t.Fatalf("seed choice_prompt: %v", err)
	}

	if err := h.svc.SubmitChoice(context.Background(), h.session, "user-1", prompt.ID, "A", "",
		func(string) error { return nil },
		func(ChoicePrompt) error { return nil },
		func(TenderResults) error { return nil },
		func(ToolCall) error { return nil },
		h.usageCh,
	); err != nil {
		t.Fatalf("SubmitChoice: %v", err)
	}

	got := h.provider.promptAt(t, 0)
	assertProviderSafe(t, got)
	if len(got) != 3 {
		t.Fatalf("prompt = %s, want [system, assistant question, user answer]", renderPrompt(got))
	}
	if got[1].Role != providers.RoleAssistant || !strings.Contains(got[1].Content, "Privato o condiviso?") {
		t.Fatalf("prompt[1] = %+v, want the question as the assistant's", got[1])
	}
	if !strings.Contains(got[1].Content, "A) Privato") {
		t.Fatalf("prompt[1].Content = %q, want the option list so the answer %q is interpretable", got[1].Content, "A) Privato")
	}
	if got[2].Role != providers.RoleUser || got[2].Content != "A) Privato" {
		t.Fatalf("prompt[2] = %+v, want the answer as the user's", got[2])
	}
}

// ── the credits ledger ──────────────────────────────────────────────────────

// TestRunTurn_AbortedTurnStillReportsUsage is the pin for the leak: a turn that
// failed after the provider answered used to return without ever touching
// usageCh, so the company paid Fireworks and the user paid nothing. Closing
// the browser tab mid-stream reproduced it on demand.
//
// A failing sendToken is the same shape the real abort takes — the gRPC send
// fails once the client is gone, consumeStream returns that error, and with no
// pending choice runTurn treats it as a genuine failure.
func TestRunTurn_AbortedTurnStillReportsUsage(t *testing.T) {
	reply := "Ho trovato tre bandi che corrispondono ai tuoi criteri."
	h := newTurnHarness(t, reply)

	disconnected := errors.New("client disconnected")
	err := h.sendWith("cerca bandi per cestini intelligenti", func(string) error { return disconnected })
	if !errors.Is(err, disconnected) {
		t.Fatalf("ChatStream err = %v, want the stream failure to propagate", err)
	}

	usage := h.usage(t)
	h.assertNoMoreUsage(t)

	if usage.SessionID != h.session.ID || usage.AgentType != string(AgentTypeBaseChat) {
		t.Fatalf("usage = %+v, want it attributed to this session and agent", usage)
	}
	if usage.TotalTokens < 1 {
		t.Fatalf("usage.TotalTokens = %d, want a non-zero charge for a turn the provider already answered", usage.TotalTokens)
	}
	// The reply reached this process before the send failed, so it is part of
	// what the turn cost — a turn that produced 2KB must not bill like one that
	// produced nothing.
	if usage.OutputTokens != estimateTokens(reply) {
		t.Fatalf("usage.OutputTokens = %d, want %d (whatever was streamed before the failure)", usage.OutputTokens, estimateTokens(reply))
	}
}

// TestRunTurn_ProviderFailureBeforeAnyOutputStillReportsUsage covers the other
// failure shape and the worst-looking one: the provider dies before returning a
// single chunk, so the user saw nothing. The request was still sent, so the
// turn still bills — at the estimate floor rather than at zero.
func TestRunTurn_ProviderFailureBeforeAnyOutputStillReportsUsage(t *testing.T) {
	h := newTurnHarness(t)
	h.provider.recvErr = errors.New("upstream reset the connection")

	if err := h.sendWith("ciao", func(string) error { return nil }); err == nil {
		t.Fatal("ChatStream err = nil, want the provider failure to propagate")
	}

	usage := h.usage(t)
	h.assertNoMoreUsage(t)
	if usage.OutputTokens != estimateTokens("") {
		t.Fatalf("usage.OutputTokens = %d, want the floor %d for a turn that produced nothing", usage.OutputTokens, estimateTokens(""))
	}
	if usage.TotalTokens < 1 {
		t.Fatalf("usage.TotalTokens = %d, want a non-zero charge", usage.TotalTokens)
	}
}

// TestRunTurn_SuccessfulTurnBillsTheUserMessageAndReplyOnly pins the half of
// this change that is deliberately NOT a change. The billing basis stays
// len(user message)/4 + len(reply)/4 even though the prompt now carries the
// whole transcript: turn 2 is charged for its own message only, exactly as
// turn 1 was. Charging for the real prompt would multiply every customer's
// bill by an unmeasured factor, which is a pricing decision and not this
// commit's to make — see turnMetadata, which measures it instead.
func TestRunTurn_SuccessfulTurnBillsTheUserMessageAndReplyOnly(t *testing.T) {
	reply1 := "Il bando scade il 30 settembre e vale 500.000 EUR."
	reply2 := "Sì, la stazione appaltante è il Comune di Milano."
	h := newTurnHarness(t, reply1, reply2)

	msg1 := "parlami del primo bando"
	h.send(t, msg1)
	u1 := h.usage(t)
	h.assertNoMoreUsage(t)

	if u1.InputTokens != estimateTokens(msg1) || u1.OutputTokens != estimateTokens(reply1) {
		t.Fatalf("turn 1 usage = %+v, want input %d / output %d", u1, estimateTokens(msg1), estimateTokens(reply1))
	}
	if u1.TotalTokens != u1.InputTokens+u1.OutputTokens {
		t.Fatalf("turn 1 total = %d, want input+output = %d", u1.TotalTokens, u1.InputTokens+u1.OutputTokens)
	}

	msg2 := "chi lo ha pubblicato?"
	h.send(t, msg2)
	u2 := h.usage(t)

	// Turn 2's prompt is [system, msg1, reply1, msg2] — several times the size
	// of turn 1's — and its bill is still only its own message.
	if u2.InputTokens != estimateTokens(msg2) {
		t.Fatalf("turn 2 InputTokens = %d, want %d; the billing basis must not have changed with the prompt", u2.InputTokens, estimateTokens(msg2))
	}
	if u2.OutputTokens != estimateTokens(reply2) {
		t.Fatalf("turn 2 OutputTokens = %d, want %d", u2.OutputTokens, estimateTokens(reply2))
	}
}

// TestRunTurn_StampsRealPromptSizeNextToTheBilledEstimate is the
// instrumentation half: the reply row records how big the prompt actually was
// beside what the turn actually charged, so the pricing decision can later be
// made on data instead of on an argument.
func TestRunTurn_StampsRealPromptSizeNextToTheBilledEstimate(t *testing.T) {
	reply1 := "Ecco i dettagli del bando che hai chiesto, con criteri e scadenze."
	reply2 := "Confermo, la scadenza è il 30 settembre."
	h := newTurnHarness(t, reply1, reply2)

	msg1 := "parlami del bando per i cestini intelligenti di Milano"
	h.send(t, msg1)
	u1 := h.usage(t)
	md1 := h.assistantMetadata(t, 0)

	if md1.EstBilledTokens != u1.TotalTokens {
		t.Fatalf("stamped est_billed_tokens = %d, want %d (what the turn actually charged)", md1.EstBilledTokens, u1.TotalTokens)
	}
	// The instructions alone dwarf the user's message, so even on turn 1 the
	// prompt is far bigger than what was billed for it. That gap is the whole
	// reason the key exists.
	if md1.CtxChars <= len(msg1) {
		t.Fatalf("stamped ctx_chars = %d, want more than the %d characters of the user message alone (the instructions are part of the prompt too)", md1.CtxChars, len(msg1))
	}

	msg2 := "e la scadenza?"
	h.send(t, msg2)
	u2 := h.usage(t)
	md2 := h.assistantMetadata(t, 1)

	if md2.EstBilledTokens != u2.TotalTokens {
		t.Fatalf("turn 2 est_billed_tokens = %d, want %d", md2.EstBilledTokens, u2.TotalTokens)
	}
	// The measurement has to track the transcript, or it cannot show the thing
	// it exists to show: the prompt grows every turn while the bill does not.
	if md2.CtxChars <= md1.CtxChars {
		t.Fatalf("turn 2 ctx_chars = %d, want more than turn 1's %d — the prompt grew by a whole exchange", md2.CtxChars, md1.CtxChars)
	}
	if md2.CtxChars < len(msg1)+len(reply1)+len(msg2) {
		t.Fatalf("turn 2 ctx_chars = %d, want at least the %d characters of the transcript it sent", md2.CtxChars, len(msg1)+len(reply1)+len(msg2))
	}
	if int(md2.EstBilledTokens)*4 >= md2.CtxChars {
		t.Fatalf("billed %d tokens against a %d-character prompt; the stamp is supposed to expose a gap, so a test where there is none is measuring the wrong thing", md2.EstBilledTokens, md2.CtxChars)
	}
}

// TestNoPerSessionStateOnServiceOrRegistry is weak as a test and strong as a
// guard: it fails the moment someone reintroduces a cross-request cache keyed
// by session id. A map with a *named* key type (Registry.configs, keyed by
// AgentType) is a closed vocabulary and stays allowed; a map keyed by plain
// string is an id-keyed cache, which is precisely the shape that made a
// session's context depend on which replica served it.
func TestNoPerSessionStateOnServiceOrRegistry(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(Service{}), reflect.TypeOf(Registry{})} {
		for i := range typ.NumField() {
			f := typ.Field(i)
			if f.Type.Kind() == reflect.Map && f.Type.Key() == reflect.TypeOf("") {
				t.Fatalf("%s.%s is a %s — an id-keyed cache on a type shared across requests; the conversation belongs in Postgres", typ.Name(), f.Name, f.Type)
			}
			if strings.Contains(f.Type.String(), "chat.Chat") {
				t.Fatalf("%s.%s holds a berrygem chat.Chat — that is the second, lossy copy of the conversation this design removed", typ.Name(), f.Name)
			}
		}
	}
}

// TestChatStream_RejectsAnEmptyMessage stops a row from being written that the
// user can see and the model never can.
//
// conversationForTurn drops any message whose mapped content is empty — it has
// to, because an empty user turn is exactly what an OpenAI-compatible backend
// is handed as a blank message. So persisting one produces a row that is
// durable, returned by GetMessages, and invisible to every subsequent turn's
// prompt: the transcript the user reads and the transcript the model reads stop
// being the same conversation, permanently and silently.
//
// The turn that sends it is wrong too. With the new message dropped, the prompt
// is the prior history verbatim, and the likeliest reply is the previous answer
// a second time — a confident non-sequitur rather than a refusal.
func TestChatStream_RejectsAnEmptyMessage(t *testing.T) {
	for _, message := range []string{"", " ", "\n\t "} {
		h := newTurnHarness(t, "should never be produced")

		err := h.sendWith(message, func(string) error { return nil })
		if !errors.Is(err, ErrEmptyMessage) {
			t.Fatalf("ChatStream(%q): err = %v, want ErrEmptyMessage", message, err)
		}
		// Rejected before the insert, so a refused turn leaves no trace: a
		// persisted-then-invisible row is the whole harm.
		msgs, err := h.repo.ListMessagesBySession(context.Background(), h.session.ID)
		if err != nil {
			t.Fatalf("ListMessagesBySession: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("ChatStream(%q) persisted %d messages, want 0", message, len(msgs))
		}
		if got := h.provider.callCount(); got != 0 {
			t.Errorf("ChatStream(%q) reached the provider %d times, want 0", message, got)
		}
	}
}

// TestGetChatForChoice_SurvivesATiedTenderResultsRow covers the ordering tie
// that used to make a genuinely pending choice permanently unanswerable.
//
// ListMessagesBySession orders by (created_at, id). created_at is now(), i.e.
// transaction-start time at microsecond resolution, and the id is a random
// uuid — so a tender_results row and the choice_prompt of the same turn can
// share a timestamp and be ordered by a coin flip. Requiring the prompt to be
// the STRICTLY last row meant losing that flip returned ErrChoiceNotPending
// from then on, with nothing the user could do to change the row order.
//
// Only a user turn can answer a choice, so only an answer or a fresh user
// message retires one. This fake preserves insertion order, so the tender
// results row landing after the prompt is exactly the losing flip.
func TestGetChatForChoice_SurvivesATiedTenderResultsRow(t *testing.T) {
	chatRepo := newFakeChatRepo()
	members := newFakeMemberRepo()
	members.allow("ws-1", "user-1")
	svc := newTestService(chatRepo, members, newFakeWorkbenches())

	session, _ := chatRepo.CreateSession(context.Background(), "user-1", "ws-1", "", "base-chat", "Test")
	prompt, err := chatRepo.InsertMessage(context.Background(), session.ID, string(roleChoicePrompt),
		"Privato o condiviso?", json.RawMessage(`[{"key":"A","label":"Privato"}]`), nil, nil)
	if err != nil {
		t.Fatalf("seed choice_prompt: %v", err)
	}
	if _, err := chatRepo.InsertMessage(context.Background(), session.ID, string(roleTenderResults),
		"", nil, nil, json.RawMessage(`[{"id":"TED-1"}]`)); err != nil {
		t.Fatalf("seed tender_results: %v", err)
	}

	if _, err := svc.GetChatForChoice(context.Background(), "user-1", prompt.ID); err != nil {
		t.Fatalf("GetChatForChoice: %v — a row from the prompt's own turn is not an answer to it", err)
	}
}

// TestGetChatForChoice_RejectsAFreshUserMessage keeps the fix above from
// widening into "a choice is pending forever". Typing something else instead of
// answering retires the prompt just as definitively as answering it.
func TestGetChatForChoice_RejectsAFreshUserMessage(t *testing.T) {
	chatRepo := newFakeChatRepo()
	members := newFakeMemberRepo()
	members.allow("ws-1", "user-1")
	svc := newTestService(chatRepo, members, newFakeWorkbenches())

	session, _ := chatRepo.CreateSession(context.Background(), "user-1", "ws-1", "", "base-chat", "Test")
	prompt, err := chatRepo.InsertMessage(context.Background(), session.ID, string(roleChoicePrompt),
		"Privato o condiviso?", json.RawMessage(`[{"key":"A","label":"Privato"}]`), nil, nil)
	if err != nil {
		t.Fatalf("seed choice_prompt: %v", err)
	}
	if _, err := chatRepo.InsertMessage(context.Background(), session.ID, string(roleUser),
		"lascia stare, cercane altri", nil, nil, nil); err != nil {
		t.Fatalf("seed user message: %v", err)
	}

	if _, err := svc.GetChatForChoice(context.Background(), "user-1", prompt.ID); !errors.Is(err, ErrChoiceNotPending) {
		t.Fatalf("GetChatForChoice: err = %v, want ErrChoiceNotPending", err)
	}
}

// TestTurnMetadata_CarriesTheInvariantsProof asserts the four fields the
// post-fix production queries are written against, because a stamp missing one
// of them is a stamp that cannot prove anything.
//
// ctx_msgs is the load-bearing one: it lets a single query recompute, from the
// rows themselves, how much of its own session each turn actually saw — the
// only evidence there will be that this defect is gone, since nothing recorded
// what any pre-fix turn was shown. pod answers the cross-pod-session question
// that no verifier could observe without cluster access.
func TestTurnMetadata_CarriesTheInvariantsProof(t *testing.T) {
	h := newTurnHarness(t, "a1", "a2")

	h.send(t, "u1")
	h.send(t, "u2")

	msgs, err := h.repo.ListMessagesBySession(context.Background(), h.session.ID)
	if err != nil {
		t.Fatalf("ListMessagesBySession: %v", err)
	}

	var stamped int
	for _, m := range msgs {
		if chatRole(m.Role) != roleAssistant {
			continue
		}
		stamped++
		if m.Metadata == nil {
			t.Fatalf("assistant row %s carries no metadata", m.ID)
		}
		var got turnMetadata
		if err := json.Unmarshal(*m.Metadata, &got); err != nil {
			t.Fatalf("metadata is not turnMetadata JSON: %v (%s)", err, string(*m.Metadata))
		}
		if got.CtxMsgs < 1 {
			t.Errorf("ctx_msgs = %d, want at least the turn's own user message", got.CtxMsgs)
		}
		if got.CtxChars <= 0 {
			t.Errorf("ctx_chars = %d, want the size of the prompt that was sent", got.CtxChars)
		}
		if got.EstBilledTokens <= 0 {
			t.Errorf("est_billed_tokens = %d, want what the turn charged", got.EstBilledTokens)
		}
		if got.Pod != "test-pod" {
			t.Errorf("pod = %q, want the process identity NewService was given", got.Pod)
		}
	}
	if stamped != 2 {
		t.Fatalf("stamped %d assistant rows, want 2", stamped)
	}

	// The second turn saw strictly more of its session than the first: that
	// growth IS the fix, and it is the number query 4 watches for cost.
	first, second := turnMetadataOf(t, msgs, 0), turnMetadataOf(t, msgs, 1)
	if second.CtxMsgs <= first.CtxMsgs {
		t.Errorf("ctx_msgs went %d -> %d; a later turn must see more of its own session", first.CtxMsgs, second.CtxMsgs)
	}
}

// turnMetadataOf decodes the nth assistant row's metadata stamp.
func turnMetadataOf(t *testing.T, msgs []postgres.DBChatMessage, n int) turnMetadata {
	t.Helper()
	var seen int
	for _, m := range msgs {
		if chatRole(m.Role) != roleAssistant {
			continue
		}
		if seen == n {
			var out turnMetadata
			if err := json.Unmarshal(*m.Metadata, &out); err != nil {
				t.Fatalf("metadata: %v", err)
			}
			return out
		}
		seen++
	}
	t.Fatalf("no assistant row at index %d", n)
	return turnMetadata{}
}
