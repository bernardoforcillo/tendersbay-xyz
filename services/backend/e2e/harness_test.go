// Package e2e drives this service the way a client does: over HTTP, through the
// ConnectRPC handlers and the middleware, against a real PostgreSQL.
//
// Everything below the transport is the production wiring — authlayer's drops
// stores, the scope and invite engines, featurelayer's entitlement engine, the
// domain services from internal/core — assembled the same way main.go assembles
// it. Only three things are substituted, and each for a reason that has nothing
// to do with the behaviour under test: the mailer captures links instead of
// sending them (the tests need to read the token a user would click), the rate
// limiter always allows (its own limits are tested elsewhere and would make
// these flaky), and there is no Qdrant, Redis or agent provider, because no
// journey here touches them.
//
// What this covers that nothing else does: the unit tests run each domain over
// an in-memory store, and the store contracts run each store without a domain.
// A permission that the domain computes correctly and the transport then drops,
// or a membership row the migration wrote in a shape the query cannot read, is
// invisible to both. These tests are where the layers meet.
package e2e

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	dropsstore "github.com/bernardoforcillo/authlayer/store/drops"
	agentv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/agent/v1/agentv1connect"
	authv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/auth/v1/authv1connect"
	bidv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/bid/v1/bidv1connect"
	companyv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/company/v1/companyv1connect"
	espdv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/espd/v1/espdv1connect"
	userv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/user/v1/userv1connect"
	workbenchv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workbench/v1/workbenchv1connect"
	workspacev1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workspace/v1/workspacev1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/connectapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/edm21"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/edm4"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/pdf"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/user"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// stack is one running service. Clients are not shared: the refresh token is an
// HttpOnly cookie, so each account gets its own jar the way each user gets their
// own browser — sharing one would have the second login overwrite the first's
// session, which is a bug in the test, not in the service.
type stack struct {
	url  string
	mail *mailbox
	// sqlDB is the one back door these tests have, and it exists for exactly
	// one thing: moving a workspace onto a paid plan. The product has no
	// upgrade RPC yet — billing is not built — so a journey that needs a Pro
	// workspace has no front door to walk through. See upgradeToPro.
	sqlDB *sql.DB

	// anon is a client with no jar and no token, for the calls a signed-out
	// visitor makes.
	anon clients
}

// clients is one caller's view of the API.
type clients struct {
	auth      authv1connect.AuthServiceClient
	user      userv1connect.UserServiceClient
	workspace workspacev1connect.WorkspaceServiceClient
	workbench workbenchv1connect.WorkbenchServiceClient
	agent     agentv1connect.AgentServiceClient
	company   companyv1connect.CompanyServiceClient
	bid       bidv1connect.BidServiceClient
	espd      espdv1connect.EspdServiceClient
	jar       http.CookieJar
	url       string
}

// newClients opens a fresh session — its own cookie jar, so its own refresh
// token.
func (s *stack) newClients(t *testing.T) clients {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	hc := &http.Client{Jar: jar}
	return clients{
		auth:      authv1connect.NewAuthServiceClient(hc, s.url),
		user:      userv1connect.NewUserServiceClient(hc, s.url),
		workspace: workspacev1connect.NewWorkspaceServiceClient(hc, s.url),
		workbench: workbenchv1connect.NewWorkbenchServiceClient(hc, s.url),
		agent:     agentv1connect.NewAgentServiceClient(hc, s.url),
		company:   companyv1connect.NewCompanyServiceClient(hc, s.url),
		bid:       bidv1connect.NewBidServiceClient(hc, s.url),
		espd:      espdv1connect.NewEspdServiceClient(hc, s.url),
		jar:       jar,
		url:       s.url,
	}
}

// cookie reports what the jar holds under a name, empty when it holds nothing —
// which is itself worth asserting after a logout.
func (c clients) cookie(name string) string {
	u, err := url.Parse(c.url)
	if err != nil {
		return ""
	}
	for _, ck := range c.jar.Cookies(u) {
		if ck.Name == name {
			return ck.Value
		}
	}
	return ""
}

// refreshCookie is cookie("refresh_token") where its absence means the test has
// already gone wrong — how a test captures a token it means to replay later.
func (c clients) refreshCookie(t *testing.T) string {
	t.Helper()
	v := c.cookie("refresh_token")
	if v == "" {
		t.Fatal("no refresh_token cookie is held")
	}
	return v
}

// setRefreshCookie plants a token in the jar, so a test can present one the
// service has already rotated past.
func (c clients) setRefreshCookie(t *testing.T, value string) {
	t.Helper()
	u, err := url.Parse(c.url)
	if err != nil {
		t.Fatalf("parse %q: %v", c.url, err)
	}
	c.jar.SetCookies(u, []*http.Cookie{{Name: "refresh_token", Value: value, Path: "/"}})
}

// newStack boots the service against TEST_DATABASE_URL. Every test gets its own
// httptest server but shares the database, so fixtures are made unique by
// uniq() rather than by truncation — these tests only ever add rows, which is
// what lets them run alongside the rest of the suite.
func newStack(t *testing.T) *stack {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	db, sqlDB, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	mail := &mailbox{}
	userRepo := postgres.NewUserRepo(db)
	authCfg := auth.Config{
		JWTSecret:     "e2e-secret-that-is-long-enough-to-sign-with",
		JWTExpiry:     time.Hour,
		RefreshExpiry: 24 * time.Hour,
		AppBaseURL:    "https://e2e.test",
	}
	authSvc := auth.NewService(dropsstore.NewAuthStore(db), userRepo, mail, allowAll{}, authCfg)

	workspaceSvc := workspace.NewService(
		workspace.NewAccess(),
		postgres.NewWorkspaceScopeStore(db),
		postgres.NewWorkspaceInviteStore(db),
		postgres.NewWorkspaceRepo(db),
		userRepo, mail,
		workspace.Config{AppBaseURL: authCfg.AppBaseURL, InviteExpiry: 7 * 24 * time.Hour},
	)
	workbenchSvc := workbench.NewService(
		workbench.NewAccess(),
		workspaceSvc.Scope(),
		postgres.NewWorkbenchScopeStore(db),
		postgres.NewWorkbenchRepo(db),
		userRepo,
		workspaceAccess{workspaceSvc},
	)

	subscriptionRepo := postgres.NewSubscriptionRepo(db)
	featureEngine, err := features.New(subscriptionRepo, postgres.NewFeatureUsageRepo(db))
	if err != nil {
		t.Fatalf("features.New: %v", err)
	}
	creditSvc := credits.NewService(featureEngine, subscriptionRepo,
		postgres.NewAgentPricingRepo(db), postgres.NewTokenUsageRepo(db))

	// The company dossier and the ESPD generator. companySvc's tender and
	// document ports are nil because only CheckEligibility reads them and no
	// journey here runs it; the dossier writes these tests drive do not touch
	// them.
	companyRepo := postgres.NewCompanyRepo(db)
	companySvc := company.NewService(companyRepo, postgres.NewRequirementRepo(db), workspaceSvc, nil, nil)

	bidRepo := postgres.NewBidRepo(db)
	bidSvc := bid.NewService(bidRepo, workbenchSvc, stubTenders{}, companySvc)

	espdStore := postgres.NewEspdStore(db)
	espdSvc, err := espd.NewService(
		companyRepo, bidRepo, espdStore, espdStore,
		workbenchSvc, stubTenders{}, featureEngine,
		[]espd.Serializer{edm21.New(), edm4.New()}, pdf.New(),
	)
	if err != nil {
		t.Fatalf("espd.NewService: %v", err)
	}

	mux := http.NewServeMux()
	for _, h := range []struct {
		path    string
		handler http.Handler
	}{
		mount(authv1connect.NewAuthServiceHandler(connectapi.NewAuthHandler(authSvc, int(authCfg.RefreshExpiry.Seconds())))),
		mount(userv1connect.NewUserServiceHandler(connectapi.NewUserHandler(
			user.NewService(authSvc.Authlayer(), userRepo, mail, authCfg)))),
		mount(workspacev1connect.NewWorkspaceServiceHandler(connectapi.NewWorkspaceHandler(workspaceSvc, creditSvc, nil))),
		mount(workbenchv1connect.NewWorkbenchServiceHandler(connectapi.NewWorkbenchHandler(workbenchSvc))),
		// The agent service itself is nil: every chat RPC needs a model
		// provider, and no journey here has one. GetCredits does not touch it —
		// it reads the membership and the entitlement — which is the one thing
		// on this handler the entitlement flow has to go through.
		mount(agentv1connect.NewAgentServiceHandler(connectapi.NewAgentHandler(nil, creditSvc, workspaceSvc))),
		mount(companyv1connect.NewCompanyServiceHandler(connectapi.NewCompanyHandler(companySvc))),
		mount(bidv1connect.NewBidServiceHandler(connectapi.NewBidHandler(bidSvc))),
		mount(espdv1connect.NewEspdServiceHandler(connectapi.NewEspdHandler(espdSvc))),
	} {
		mux.Handle(h.path, h.handler)
	}

	// The same middleware chain main.go wraps the mux in. JWTMiddleware is the
	// one that matters here: without it every authenticated call is anonymous,
	// and the authorization these tests are about never runs.
	srv := httptest.NewServer(connectapi.JWTMiddleware(authSvc)(connectapi.ClientIPMiddleware(mux)))
	t.Cleanup(srv.Close)

	s := &stack{url: srv.URL, mail: mail, sqlDB: sqlDB}
	s.anon = s.newClients(t)
	return s
}

func mount(path string, h http.Handler) struct {
	path    string
	handler http.Handler
} {
	return struct {
		path    string
		handler http.Handler
	}{path, h}
}

// ── the three substitutions ─────────────────────────────────────────────────

// mailbox captures what would have been sent. The verification and invitation
// links carry the only copy of their token — the service hands it to the user
// through the mail and keeps only a hash — so reading them here is the only way
// to complete the journey a real user completes.
type mailbox struct {
	mu   sync.Mutex
	sent []sentMail
}

type sentMail struct{ kind, to, link string }

func (m *mailbox) record(kind, to, link string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{kind, to, link})
	return nil
}

func (m *mailbox) SendVerification(_ context.Context, to, _, link string) error {
	return m.record("verification", to, link)
}

func (m *mailbox) SendPasswordReset(_ context.Context, to, _, link string) error {
	return m.record("password_reset", to, link)
}

func (m *mailbox) SendEmailChangeVerification(_ context.Context, to, _, link string) error {
	return m.record("email_change", to, link)
}

func (m *mailbox) SendAccountExists(_ context.Context, to, _ string) error {
	return m.record("account_exists", to, "")
}

func (m *mailbox) SendWorkspaceInvite(_ context.Context, to, _, _, link string) error {
	return m.record("workspace_invite", to, link)
}

// lastTo returns the token in the most recent mail of a kind sent to an
// address, failing the test if there is none — an assertion in its own right,
// since a journey that expected a mail and got none has already gone wrong.
func (m *mailbox) lastTo(t *testing.T, kind, to string) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.sent) - 1; i >= 0; i-- {
		s := m.sent[i]
		if s.kind == kind && strings.EqualFold(s.to, to) {
			return tokenFrom(t, s.link)
		}
	}
	t.Fatalf("no %s mail was sent to %s (sent: %+v)", kind, to, m.sent)
	return ""
}

func (m *mailbox) countTo(kind, to string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.sent {
		if s.kind == kind && strings.EqualFold(s.to, to) {
			n++
		}
	}
	return n
}

// tokenFrom pulls the token out of a link, which is how the client gets it:
// the user clicks, the SPA reads the query parameter, and sends it back.
func tokenFrom(t *testing.T, link string) string {
	t.Helper()
	_, query, ok := strings.Cut(link, "?")
	if !ok {
		t.Fatalf("link %q carries no query", link)
	}
	for _, part := range strings.Split(query, "&") {
		if k, v, found := strings.Cut(part, "="); found && (k == "token" || k == "code") {
			return v
		}
	}
	t.Fatalf("link %q carries no token", link)
	return ""
}

// allowAll stands in for the Redis rate limiter. Its own behaviour is covered
// in internal/core/auth; here it would only make a test that logs in twice
// flaky.
type allowAll struct{}

func (allowAll) Allow(context.Context, string, int64, time.Duration) (bool, error) {
	return true, nil
}

// workspaceAccess is main.go's adapter, repeated because it lives in package
// main and cannot be imported. Keeping it identical is the point: the nesting's
// view of the workspace is what these tests exercise.
type workspaceAccess struct{ svc *workspace.Service }

func (w workspaceAccess) Lookup(ctx context.Context, workspaceID, userID string) (workbench.WorkspaceInfo, error) {
	st, err := w.svc.Standing(ctx, workspaceID, userID)
	if err != nil {
		return workbench.WorkspaceInfo{}, err
	}
	return workbench.WorkspaceInfo{
		IsMember:      st.IsMember,
		MayViewShared: st.Permissions.Has(workspace.PermViewWorkbenches),
		MayManageAll:  st.Permissions.Has(workspace.PermManageWorkbenches),
	}, nil
}

func (w workspaceAccess) WorkspaceName(ctx context.Context, workspaceID string) (string, error) {
	ws, err := w.svc.Scope().Container(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	return ws.Name, nil
}

// ── request helpers ─────────────────────────────────────────────────────────

// authed stamps the bearer token the JWT middleware reads.
func authed[T any](msg *T, token string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+token)
	return req
}

// uniq makes a fixture name that cannot collide with another run's, since these
// tests share a database nobody truncates.
func uniq(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// codeOf reports a Connect error's code, so a test can assert on the wire
// status a client actually sees rather than on a sentinel the transport was
// supposed to translate. A nil or non-Connect error reports CodeUnknown, which
// no handler returns deliberately.
func codeOf(err error) connect.Code {
	var cerr *connect.Error
	if errors.As(err, &cerr) {
		return cerr.Code()
	}
	return connect.CodeUnknown
}

// stubTenders is the FOURTH substitution, alongside the mailer, the rate
// limiter and the absent agent provider.
//
// The tender corpus lives in the `tenders` schema, which services/ingestion
// owns and migrates; a backend e2e that needed real notices would have to seed
// another service's tables, which is exactly the coupling the two migration
// chains exist to avoid. What these journeys need from a tender is Part I —
// who is buying, under which reference — so that is what this returns, keyed by
// the id the bid carries.
//
// It is deliberately NOT a fake with behaviour: no fit scoring, no
// availability. A test that needed those would be testing the tender domain,
// which has its own tests.
type stubTenders struct{}

func (stubTenders) GetTender(_ context.Context, p tender.GetTenderParams) (tender.TenderDetail, error) {
	return tender.TenderDetail{
		ID:                p.ID,
		Title:             "Manutenzione strade comunali 2027",
		BuyerName:         "Comune di Milano",
		SourceRef:         "CIG" + p.ID,
		PublicationNumber: "2026/S 100-" + p.ID,
		Country:           "IT",
		Status:            "open",
	}, nil
}

func (stubTenders) SummariesByIDs(_ context.Context, ids []int64) (map[int64]tender.TenderSummary, error) {
	out := make(map[int64]tender.TenderSummary, len(ids))
	for _, id := range ids {
		out[id] = tender.TenderSummary{
			ID: id, Title: "Manutenzione strade comunali 2027",
			BuyerName: "Comune di Milano", Country: "IT", Status: "open",
		}
	}
	return out, nil
}

func (stubTenders) FitForTenders(_ context.Context, _, _ string, ids []int64) (map[int64]tender.TenderFitResult, error) {
	return map[int64]tender.TenderFitResult{}, nil
}

// upgradeToPro moves a workspace onto the Pro plan.
//
// It writes the subscription row directly because there is no RPC that does
// this: the product has no billing flow yet, and CreateWorkspace seeds every
// workspace on Free. Everything downstream of the row — featurelayer reading
// it back through the Postgres store, resolving the plan's entitlements — is
// exercised for real, which is the part these journeys are about.
func (s *stack) upgradeToPro(t *testing.T, workspaceID string) {
	t.Helper()
	res, err := s.sqlDB.ExecContext(context.Background(),
		`UPDATE workspace_subscriptions SET plan = $1, updated_at = now() WHERE workspace_id = $2`,
		string(features.PlanPro), workspaceID)
	if err != nil {
		t.Fatalf("upgrade %s to pro: %v", workspaceID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if n != 1 {
		t.Fatalf("upgrading %s touched %d rows; CreateWorkspace should have seeded exactly one", workspaceID, n)
	}
}
