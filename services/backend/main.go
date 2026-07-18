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

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/telemetry"
	"github.com/joho/godotenv"

	agentv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/agent/v1/agentv1connect"
	authv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/auth/v1/authv1connect"
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
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
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

	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	db, sqlDB, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	userRepo := postgres.NewUserRepo(db)
	sessionRepo := postgres.NewSessionRepo(db)
	evRepo := postgres.NewEVRepo(db)
	prRepo := postgres.NewPRRepo(db)

	workspaceRepo := postgres.NewWorkspaceRepo(db)
	roleRepo := postgres.NewRoleRepo(db)
	memberRepo := postgres.NewMemberRepo(db)
	emailInviteRepo := postgres.NewEmailInviteRepo(db)
	inviteLinkRepo := postgres.NewInviteLinkRepo(db)
	workspaceUow := postgres.NewUnitOfWork(db)

	// Client profile (per-client bid-qualification agent, v1.0) — built here,
	// before both tenderSvc and agentSvc, since tenderSvc.RecommendForClient
	// needs it as a ProfileSource.
	clientProfileRepo := postgres.NewClientProfileRepo(db)
	clientProfileSvc := clientprofile.NewService(clientProfileRepo, memberRepo)

	var mailer interface {
		SendVerification(ctx context.Context, to, displayName, link string) error
		SendPasswordReset(ctx context.Context, to, displayName, link string) error
		SendEmailChangeVerification(ctx context.Context, to, displayName, link string) error
		SendWorkspaceInvite(ctx context.Context, to, workspaceName, inviterName, link string) error
	}
	if cfg.ResendAPIKey == "" {
		slog.Warn("RESEND_API_KEY not set — emails will be logged to stdout only")
		mailer = email.NewLog()
	} else {
		mailer = email.NewResend(cfg.ResendAPIKey, "noreply@tendersbay.xyz")
	}

	authCfg := auth.Config{
		JWTSecret:     cfg.JWTSecret,
		JWTExpiry:     cfg.JWTExpiry,
		RefreshExpiry: cfg.RefreshExpiry,
		AppBaseURL:    cfg.AppBaseURL,
	}

	authSvc := auth.NewService(userRepo, sessionRepo, evRepo, prRepo, mailer, authCfg)
	userSvc := user.NewService(userRepo, sessionRepo, evRepo, mailer, authCfg)
	workspaceSvc := workspace.NewService(
		workspaceRepo, roleRepo, memberRepo, emailInviteRepo, inviteLinkRepo,
		userRepo, mailer, workspaceUow,
		workspace.Config{AppBaseURL: cfg.AppBaseURL, InviteExpiry: cfg.WorkspaceInviteExpiry},
	)

	workbenchWSAccess := postgres.NewWorkbenchWorkspaceAccess(db)
	workbenchUow := postgres.NewWorkbenchUnitOfWork(db)
	workbenchSvc := workbench.NewService(
		postgres.NewWorkbenchRepo(db),
		postgres.NewWorkbenchRoleRepo(db),
		postgres.NewWorkbenchMemberRepo(db),
		userRepo, // satisfies workbench.UserLookup (FindByID)
		workbenchWSAccess,
		workbenchUow,
	)

	// Tender search — Qdrant/Ollama/Redis unreachable at startup is logged,
	// not fatal: search degrades to Postgres-only filtering via
	// knowledgeBaseAdapter's nil handling (for Qdrant/Ollama) or fails
	// rate-limit checks via unavailableRateLimiter (for Redis) — neither
	// blocks the whole service from starting over an optional dependency.
	// MOVED above the agent block: agentSvc's search_tenders tool needs
	// tenderSvc as its TenderSearcher.
	kb, kbErr := knowledge.NewKnowledgeBase(ctx, cfg.QdrantURL, cfg.OllamaBaseURL, cfg.EmbeddingModel)
	if kbErr != nil {
		slog.Warn("failed to connect to knowledge base, semantic search will be degraded", "error", kbErr)
	}

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

	tenderRepo := postgres.NewTenderRepo(db)
	tenderSvc := tender.NewService(
		tenderRepo,
		knowledgeBaseAdapter{kb},
		rl,
		clientProfileSvc,
		tender.Config{
			AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
			AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
			// Uncalibrated defaults — no conversion data exists pre-launch
			// (see the design spec's Risks section). Retune here, no code change.
			Fit: tender.FitThresholds{RelevanceHigh: 0.75, RelevanceLow: 0.4, MinDeadlineDays: 10, UrgentDeadlineDays: 5},
		},
	)
	tenderHandler := connectapi.NewTenderHandler(tenderSvc, memberRepo)

	// Agent / chat service
	chatRepo := postgres.NewChatRepo(db)
	creditRepo := postgres.NewWorkspaceCreditRepo(db)
	pricingRepo := postgres.NewAgentPricingRepo(db)
	usageRepo := postgres.NewTokenUsageRepo(db)

	agentRegistry := agent.NewRegistry(cfg.FireworksAPIKey)
	agentRegistry.RegisterDefaults()

	creditSvc := credits.NewService(creditRepo, pricingRepo, usageRepo)
	agentSvc := agent.NewService(agentRegistry, chatRepo, creditSvc, memberRepo, workbenchSvc, tenderSvc)

	authHandler := connectapi.NewAuthHandler(authSvc, int(cfg.RefreshExpiry.Seconds()))
	userHandler := connectapi.NewUserHandler(userSvc)
	workspaceHandler := connectapi.NewWorkspaceHandler(workspaceSvc, creditSvc, clientProfileSvc)
	workbenchHandler := connectapi.NewWorkbenchHandler(workbenchSvc)
	agentHandler := connectapi.NewAgentHandler(agentSvc, creditSvc, memberRepo)

	authPath, authRPC := authv1connect.NewAuthServiceHandler(authHandler)
	userPath, userRPC := userv1connect.NewUserServiceHandler(userHandler)
	workspacePath, workspaceRPC := workspacev1connect.NewWorkspaceServiceHandler(workspaceHandler)
	workbenchPath, workbenchRPC := workbenchv1connect.NewWorkbenchServiceHandler(workbenchHandler)
	agentPath, agentRPC := agentv1connect.NewAgentServiceHandler(agentHandler)
	tenderPath, tenderRPC := tenderv1connect.NewTenderServiceHandler(tenderHandler)

	healthSvc := health.New(probe.NewReady(), probe.NewDB(sqlDB))

	mux := http.NewServeMux()
	mux.Handle(authPath, authRPC)
	mux.Handle(userPath, userRPC)
	mux.Handle(workspacePath, workspaceRPC)
	mux.Handle(workbenchPath, workbenchRPC)
	mux.Handle(agentPath, agentRPC)
	mux.Handle(tenderPath, tenderRPC)
	mux.Handle("/", httpapi.New(healthSvc))

	handler := connectapi.NewCORS(cfg.CORSOrigins)(connectapi.JWTMiddleware(cfg.JWTSecret)(connectapi.ClientIPMiddleware(mux)))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

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

// knowledgeBaseAdapter converts *knowledge.KnowledgeBase's
// []knowledge.SearchResult into the []tender.ScoredChunk shape
// tender.KnowledgeBase expects, and turns a nil KnowledgeBase (Qdrant/Ollama
// unreachable at startup) into a clean error instead of a nil-pointer panic
// — tender.Service.Search already falls back to the filters-only path
// whenever the knowledge base returns an error.
type knowledgeBaseAdapter struct {
	kb *knowledge.KnowledgeBase
}

func (a knowledgeBaseAdapter) SearchWithScores(ctx context.Context, query string, limit int) ([]tender.ScoredChunk, error) {
	if a.kb == nil {
		return nil, errors.New("knowledge base unavailable")
	}
	results, err := a.kb.SearchWithScores(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]tender.ScoredChunk, len(results))
	for i, r := range results {
		out[i] = tender.ScoredChunk{DocID: r.DocID, Score: r.Score}
	}
	return out, nil
}

// unavailableRateLimiter denies every request with an explanatory error,
// used only when redis.NewRateLimiter itself failed (malformed REDIS_URL)
// — an actually-unreachable-but-parseable Redis is handled by
// *redis.RateLimiter.Allow's own error return instead.
type unavailableRateLimiter struct{ err error }

func (u unavailableRateLimiter) Allow(context.Context, string, int64, time.Duration) (bool, error) {
	return false, u.err
}
