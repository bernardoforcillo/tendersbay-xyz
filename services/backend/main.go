package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	dropsstore "github.com/bernardoforcillo/authlayer/store/drops"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/telemetry"
	"github.com/joho/godotenv"

	agentv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/agent/v1/agentv1connect"
	authv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/auth/v1/authv1connect"
	bidv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/bid/v1/bidv1connect"
	companyv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/company/v1/companyv1connect"
	tenderv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1/tenderv1connect"
	userv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/user/v1/userv1connect"
	workbenchv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workbench/v1/workbenchv1connect"
	workspacev1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workspace/v1/workspacev1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/connectapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/email"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/httpapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/probe"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/redis"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/config"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/agent"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/health"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/user"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

func main() {
	// Load a local .env for secrets like FIREWORKS_API_KEY that
	// scripts/run-development.sh doesn't export (gitignored; absent in
	// CI/production, where the platform injects env vars directly).
	_ = godotenv.Load()

	cfg := config.FromEnv()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := telemetry.Setup(ctx, telemetry.Config{
		APIKey:      cfg.PostHogAPIKey,
		Host:        cfg.PostHogHost,
		ServiceName: cfg.ServiceName,
	})
	if err != nil {
		slog.Error("failed to set up telemetry", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(context.Background()) }()

	// Fail closed on a missing/weak required secret (DATABASE_URL, JWT_SECRET)
	// before doing any work — an empty JWT_SECRET would otherwise let the HS256
	// signer verify forged tokens signed with an empty key.
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	db, sqlDB, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	// authStore is authlayer's own PostgreSQL store, pointed at the users,
	// sessions and verifications tables migration 0012 shaped for it. It owns
	// every credential column; userRepo below owns display_name and nothing
	// else, so no column has two writers.
	authStore := dropsstore.NewAuthStore(db)
	userRepo := postgres.NewUserRepo(db)

	var mailer interface {
		SendVerification(ctx context.Context, to, displayName, link string) error
		SendPasswordReset(ctx context.Context, to, displayName, link string) error
		SendEmailChangeVerification(ctx context.Context, to, displayName, link string) error
		SendAccountExists(ctx context.Context, to, displayName string) error
		SendWorkspaceInvite(ctx context.Context, to, workspaceName, inviterName, link string) error
	}
	if cfg.ResendAPIKey == "" {
		slog.Warn("RESEND_API_KEY not set — emails will be logged to stdout only")
		mailer = email.NewLog()
	} else {
		mailer = email.NewResend(cfg.ResendAPIKey, "noreply@tendersbay.xyz")
	}

	// Redis-backed rate limiter, shared by the auth service (login/signup/
	// forgot-password brute-force protection) and tender search. A Redis
	// failure is logged, not fatal: tender search fails its rate-limit CHECK
	// closed (unavailableRateLimiter denies), while the auth service fails its
	// checks OPEN (see auth.Service.allow) so an outage can't lock everyone out.
	var rl tender.RateLimiter
	rateLimiter, rlErr := redis.NewRateLimiter(cfg.RedisURL)
	if rlErr != nil {
		slog.Warn("failed to connect to redis, search will be rate-limited to zero", "error", rlErr)
		rl = unavailableRateLimiter{err: rlErr}
	} else {
		rl = rateLimiter
		if pingErr := rateLimiter.Ping(ctx); pingErr != nil {
			slog.Warn("redis ping failed at startup, rate limiting may be degraded", "error", pingErr)
		}
	}
	if rateLimiter != nil {
		defer rateLimiter.Close()
	}

	authCfg := auth.Config{
		JWTSecret:     cfg.JWTSecret,
		JWTExpiry:     cfg.JWTExpiry,
		RefreshExpiry: cfg.RefreshExpiry,
		AppBaseURL:    cfg.AppBaseURL,
	}

	authSvc := auth.NewService(authStore, userRepo, mailer, rl, authCfg)
	// core/user shares authSvc's underlying authlayer service rather than
	// building a second one over the same store — see user.NewService.
	userSvc := user.NewService(authSvc.Authlayer(), userRepo, mailer, authCfg)

	// Workspace RBAC and invitations run on authlayer's scope and invite
	// engines over its drops store. The access engine is built once and shared:
	// it holds the declared permission surface and the code-defined roles, and
	// two of them would be two vocabularies.
	workspaceSvc := workspace.NewService(
		workspace.NewAccess(),
		postgres.NewWorkspaceScopeStore(db),
		postgres.NewWorkspaceInviteStore(db),
		postgres.NewWorkspaceRepo(db),
		userRepo, mailer,
		workspace.Config{AppBaseURL: cfg.AppBaseURL, InviteExpiry: cfg.WorkspaceInviteExpiry},
	)

	// Feature management: what a workspace may use and how much of it. The
	// definitions live in code (internal/core/features); the two stores here
	// hold the per-workspace half — its subscription and its usage counters.
	// A failure is fatal rather than degraded: without the entitlement engine
	// every metered feature would be unmetered.
	subscriptionRepo := postgres.NewSubscriptionRepo(db)
	featureEngine, err := features.New(subscriptionRepo, postgres.NewFeatureUsageRepo(db))
	if err != nil {
		slog.Error("failed to build the feature engine", "error", err)
		os.Exit(1)
	}

	// Client profile (per-client bid-qualification agent, v1.0) — built here,
	// before both tenderSvc and agentSvc, since tenderSvc.RecommendForClient
	// needs it as a ProfileSource.
	clientProfileRepo := postgres.NewClientProfileRepo(db)
	clientProfileSvc := clientprofile.NewService(clientProfileRepo, workspaceSvc)

	// Workbenches are a NESTED authlayer scope: workspaceSvc.Scope() is the
	// parent, so "may administer every workbench" and "may create one" are
	// resolved from the workspace's own grants rather than re-derived here.
	workbenchSvc := workbench.NewService(
		workbench.NewAccess(),
		workspaceSvc.Scope(),
		postgres.NewWorkbenchScopeStore(db),
		postgres.NewWorkbenchRepo(db),
		userRepo, // satisfies workbench.UserLookup (FindByID)
		workspaceAccess{workspaceSvc},
	)

	// Tender search — Qdrant/Ollama/Redis unreachable at startup is logged,
	// not fatal: with the vector store down, knowledgeBaseAdapter's nil
	// handling makes every semantic call error, and hybrid retrieval keeps
	// answering queries from the lexical index alone (reporting the
	// degradation via SearchOutput.Mode). Redis down fails rate-limit checks
	// closed via unavailableRateLimiter. Neither blocks the whole service from
	// starting over an optional dependency.
	// MOVED above the agent block: agentSvc's search_tenders tool needs
	// tenderSvc as its TenderSearcher.
	kb, kbErr := knowledge.NewKnowledgeBase(ctx, cfg.QdrantURL, cfg.OllamaBaseURL, cfg.EmbeddingModel)
	if kbErr != nil {
		slog.Warn("failed to connect to knowledge base, semantic search will be degraded", "error", kbErr)
	}

	// Query embeddings are memoised in Redis. Embedding is an HTTP round trip
	// to Ollama on the critical path of every search — and of every page of
	// the same search. Keyed by embedding model, so switching models can never
	// serve a vector computed by the previous one. Optional: a failure here
	// only costs latency, so it's a warning, not a startup failure.
	var embeddingCache tender.EmbeddingCache
	if cache, cacheErr := redis.NewEmbeddingCache(cfg.RedisURL, cfg.EmbeddingModel, embeddingCacheTTL); cacheErr != nil {
		slog.Warn("failed to build the embedding cache, every search will re-embed its query", "error", cacheErr)
	} else {
		embeddingCache = cache
		defer cache.Close()
	}

	tenderRepo := postgres.NewTenderRepo(db)
	tenderSvc := tender.NewService(
		tenderRepo,
		knowledgeBaseAdapter{kb},
		rl,
		clientProfileSvc,
		tender.Config{
			AnonTier:      tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
			AuthedTier:    tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
			GetTenderTier: tender.Tier{MaxResults: 20, RateLimit: 600, RateWindow: time.Minute},
			// Uncalibrated defaults — no conversion data exists pre-launch
			// (see the design spec's Risks section). Retune here, no code change.
			// NOTE: RelevanceScore is now the normalised hybrid-fusion score
			// (1.0 = top of both retrievers), not a raw cosine similarity —
			// these thresholds are read on that scale. See tender.Service.fuse.
			Fit: tender.FitThresholds{RelevanceHigh: 0.75, RelevanceLow: 0.4, MinDeadlineDays: 10, UrgentDeadlineDays: 5},
			// Hybrid ranking knobs, tunable here without touching the ranking
			// logic. Uncalibrated like Fit above — there is no click data yet.
			Ranking: tender.DefaultRanking(),
			// Statutory EU procurement thresholds (2026-2027, EC), minor units.
			// A biennial revision is a one-line change here — never in the classifier.
			EU: tender.EUThreshold{
				WorksMinor:              540400000, // €5,404,000
				SuppliesCentralMinor:    14000000,  // €140,000
				SuppliesSubCentralMinor: 21600000,  // €216,000
			},
		},
	).WithEmbeddingCache(embeddingCache).
		// The CPV vocabulary is what makes a search typed in one EU language find
		// notices written in another. Attached unconditionally: a database whose
		// cpv_terms table has not been seeded yet simply resolves no codes, and
		// the arm contributes nothing — see cpvCandidates.
		WithCPVLexicon(postgres.NewCPVLexicon(db))

	// Document reading. Constructed here, above the two services that consume
	// it (company for eligibility coverage, agent for read_tender_documents),
	// rather than beside the agent alone. Its retrieval is always scoped to one
	// tender's already-extracted parts, so it needs no index this service would
	// have to create and no dependency beyond the same pool everything else uses.
	documentSvc := document.NewService(postgres.NewDocumentRepo(db))

	// The tender transport is built AFTER documentSvc because GetTenderPassages
	// serves the scheda gara's coverage strip and passage reads from it. The two
	// belong on one service for the client: a passage without the coverage that
	// qualifies it is exactly the pairing core/document refuses to break.
	tenderHandler := connectapi.NewTenderHandler(tenderSvc, workspaceSvc, documentSvc)

	// Company dossier + eligibility. tenderSvc supplies the submission deadline
	// (without it an expiring attestation reads as met) and documentSvc supplies
	// the coverage the assessment carries, so ignorance is reported as ignorance
	// rather than as a clean bill of health.
	companySvc := company.NewService(
		postgres.NewCompanyRepo(db), postgres.NewRequirementRepo(db),
		workspaceSvc, tenderSvc, documentSvc,
	)

	// Bid lifecycle (workbench-bando-hub) — consumes workbenchSvc for access
	// checks, tenderSvc for fresh fit + tender summaries, and companySvc for the
	// eligibility recommendation each go/no-go is recorded against.
	bidRepo := postgres.NewBidRepo(db)
	bidSvc := bid.NewService(bidRepo, workbenchSvc, tenderSvc, companySvc)

	// Agent / chat service
	chatRepo := postgres.NewChatRepo(db)
	pricingRepo := postgres.NewAgentPricingRepo(db)
	usageRepo := postgres.NewTokenUsageRepo(db)

	agentRegistry := agent.NewRegistry(cfg.FireworksAPIKey)
	agentRegistry.RegisterDefaults()

	// The pod name every assistant turn is stamped with. Read once, here,
	// because which process served a turn is a deployment fact and core/agent
	// must not reach for process identity itself. In Kubernetes this is the
	// pod name with no manifest edit; an error means the process could not
	// name itself, which is not a reason to refuse to start — the turns simply
	// record an empty pod, and the one query that groups by it says so.
	pod, err := os.Hostname()
	if err != nil {
		slog.Warn("could not determine the pod name; agent turns will record an empty pod", "error", err)
	}

	creditSvc := credits.NewService(featureEngine, subscriptionRepo, pricingRepo, usageRepo)
	agentSvc := agent.NewService(agentRegistry, chatRepo, creditSvc, workspaceSvc, workbenchSvc, tenderSvc, documentSvc, companySvc, clientProfileSvc, pod)

	authHandler := connectapi.NewAuthHandler(authSvc, int(cfg.RefreshExpiry.Seconds()))
	userHandler := connectapi.NewUserHandler(userSvc)
	workspaceHandler := connectapi.NewWorkspaceHandler(workspaceSvc, creditSvc, clientProfileSvc)
	workbenchHandler := connectapi.NewWorkbenchHandler(workbenchSvc)
	agentHandler := connectapi.NewAgentHandler(agentSvc, creditSvc, workspaceSvc)
	bidHandler := connectapi.NewBidHandler(bidSvc)
	companyHandler := connectapi.NewCompanyHandler(companySvc)

	authPath, authRPC := authv1connect.NewAuthServiceHandler(authHandler)
	userPath, userRPC := userv1connect.NewUserServiceHandler(userHandler)
	workspacePath, workspaceRPC := workspacev1connect.NewWorkspaceServiceHandler(workspaceHandler)
	workbenchPath, workbenchRPC := workbenchv1connect.NewWorkbenchServiceHandler(workbenchHandler)
	agentPath, agentRPC := agentv1connect.NewAgentServiceHandler(agentHandler)
	tenderPath, tenderRPC := tenderv1connect.NewTenderServiceHandler(tenderHandler)
	bidPath, bidRPC := bidv1connect.NewBidServiceHandler(bidHandler)
	companyPath, companyRPC := companyv1connect.NewCompanyServiceHandler(companyHandler)

	healthSvc := health.New(probe.NewReady(), probe.NewDB(sqlDB))

	mux := http.NewServeMux()
	mux.Handle(authPath, authRPC)
	mux.Handle(userPath, userRPC)
	mux.Handle(workspacePath, workspaceRPC)
	mux.Handle(workbenchPath, workbenchRPC)
	mux.Handle(agentPath, agentRPC)
	mux.Handle(tenderPath, tenderRPC)
	mux.Handle(bidPath, bidRPC)
	mux.Handle(companyPath, companyRPC)
	mux.Handle("/", httpapi.New(healthSvc))

	handler := connectapi.NewCORS(cfg.CORSOrigins)(connectapi.JWTMiddleware(authSvc)(connectapi.ClientIPMiddleware(mux)))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	// Housekeeping: authlayer refuses an expired session, verification or
	// invitation on read but never removes it, so without this the three tables
	// only grow. Started after everything it sweeps is built, and stopped by
	// the same context the server is.
	startHousekeeping(ctx, housekeepingInterval,
		namedPurge{"auth", authSvc.PurgeExpired},
		namedPurge{"invites", workspaceSvc.PurgeExpiredInvites},
	)

	srvErr := make(chan error, 1)
	go func() {
		slog.Info("backend listening", "addr", "http://localhost:"+cfg.Port)
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-srvErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
	}
}

// housekeepingInterval is how often the expired-row sweeps run. Hourly is far
// more often than it needs to be for correctness — nothing depends on a dead
// row being gone — and cheap enough that the tables never accumulate a backlog
// worth noticing.
const housekeepingInterval = time.Hour

// namedPurge is one sweep and the name it is logged under.
type namedPurge struct {
	name string
	run  func(ctx context.Context, before time.Time) (int, error)
}

// startHousekeeping runs each sweep on a ticker until ctx is done. It runs one
// round immediately: a process that restarts more often than the interval would
// otherwise never sweep at all.
//
// A failing sweep is logged and retried on the next tick, never fatal — falling
// behind on deleting dead rows is not a reason to take the service down. Each
// round gets its own timeout so a slow delete cannot wedge the loop.
func startHousekeeping(ctx context.Context, every time.Duration, purges ...namedPurge) {
	run := func() {
		for _, p := range purges {
			// Shutdown cancels ctx; a sweep started here would only fail on it
			// and log a warning that says nothing about the service's health.
			if ctx.Err() != nil {
				return
			}
			roundCtx, cancel := context.WithTimeout(ctx, time.Minute)
			n, err := p.run(roundCtx, time.Now().UTC())
			cancel()
			if err != nil {
				slog.WarnContext(ctx, "housekeeping sweep failed", "sweep", p.name, "error", err)
				continue
			}
			if n > 0 {
				slog.InfoContext(ctx, "housekeeping swept expired rows", "sweep", p.name, "rows", n)
			}
		}
	}
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		run()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

// embeddingCacheTTL bounds how long a memoised query embedding lives. Long
// enough that paging through one search, and repeats of a popular query,
// never re-embed; short enough that the cache's memory stays proportional to
// recent traffic rather than growing with every query ever typed.
const embeddingCacheTTL = time.Hour

// knowledgeBaseAdapter converts *knowledge.KnowledgeBase's
// []knowledge.SearchResult into the []tender.ScoredChunk shape
// tender.KnowledgeBase expects, and turns a nil KnowledgeBase (Qdrant/Ollama
// unreachable at startup) into a clean error instead of a nil-pointer panic
// — tender.Service.Search already falls back to the filters-only path
// whenever the knowledge base returns an error.
type knowledgeBaseAdapter struct {
	kb *knowledge.KnowledgeBase
}

func (a knowledgeBaseAdapter) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	if a.kb == nil {
		return nil, errors.New("knowledge base unavailable")
	}
	return a.kb.EmbedQuery(ctx, query)
}

func (a knowledgeBaseAdapter) SearchByVector(ctx context.Context, vec []float32, limit int, filters tender.Filters) ([]tender.ScoredChunk, error) {
	if a.kb == nil {
		return nil, errors.New("knowledge base unavailable")
	}
	results, err := a.kb.SearchByVector(ctx, vec, limit, vectorFilter(filters))
	if err != nil {
		return nil, err
	}
	out := make([]tender.ScoredChunk, len(results))
	for i, r := range results {
		out[i] = tender.ScoredChunk{DocID: r.DocID, Score: r.Score}
	}
	return out, nil
}

// vectorFilter maps the domain's filters onto the vector store's payload
// filter. Buyer is deliberately not carried across: buyer_name is not in the
// point payload, and this filter is only ever an optimisation — dropping a
// constraint here costs some wasted candidates, whereas approximating one
// would hide matching tenders. Postgres applies the full filter regardless.
func vectorFilter(f tender.Filters) knowledge.SearchFilter {
	return knowledge.SearchFilter{
		Countries:    f.Countries,
		Statuses:     f.Statuses,
		CPVPrefixes:  f.CPVPrefixes,
		NUTSPrefixes: f.NUTSPrefixes,
		ValueMin:     f.ValueMin,
		ValueMax:     f.ValueMax,
		DeadlineFrom: f.DeadlineFrom,
		DeadlineTo:   f.DeadlineTo,
	}
}

func (a knowledgeBaseAdapter) RelatedByDocID(ctx context.Context, docID string, limit int) ([]tender.ScoredChunk, error) {
	if a.kb == nil {
		return nil, errors.New("knowledge base unavailable")
	}
	results, err := a.kb.RelatedByDocID(ctx, docID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]tender.ScoredChunk, len(results))
	for i, r := range results {
		out[i] = tender.ScoredChunk{DocID: r.DocID, Score: r.Score}
	}
	return out, nil
}

// workspaceAccess adapts the workspace domain to workbench.WorkspaceAccess.
// It lives here, in the composition root, rather than in either domain: the
// workbench package deliberately does not import the workspace package (see
// its WorkspaceAccess doc), and the workspace package must not learn about
// workbenches to satisfy it.
type workspaceAccess struct{ svc *workspace.Service }

func (a workspaceAccess) Lookup(ctx context.Context, workspaceID, userID string) (workbench.WorkspaceInfo, error) {
	st, err := a.svc.Standing(ctx, workspaceID, userID)
	if err != nil {
		return workbench.WorkspaceInfo{}, err
	}
	return workbench.WorkspaceInfo{
		Name:          st.WorkspaceName,
		IsMember:      st.IsMember,
		MayViewShared: st.Permissions.Has(workspace.PermViewWorkbenches),
		MayManageAll:  st.Permissions.Has(workspace.PermManageWorkbenches),
	}, nil
}

// unavailableRateLimiter denies every request with an explanatory error,
// used only when redis.NewRateLimiter itself failed (malformed REDIS_URL)
// — an actually-unreachable-but-parseable Redis is handled by
// *redis.RateLimiter.Allow's own error return instead.
type unavailableRateLimiter struct{ err error }

func (u unavailableRateLimiter) Allow(context.Context, string, int64, time.Duration) (bool, error) {
	return false, u.err
}
