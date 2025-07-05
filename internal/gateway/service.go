package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/alexnthnz/webhook/config"
	"github.com/alexnthnz/webhook/internal/security"
	pb "github.com/alexnthnz/webhook/proto/generated/proto"
)

// Service implements the API Gateway HTTP service
type Service struct {
	config              config.APIGatewayConfig
	log                 *logrus.Logger
	webhookClient       pb.WebhookRegistryClient
	observabilityClient pb.ObservabilityServiceClient
	auth                *security.AuthMiddleware
	jwtManager          *security.JWTManager
	server              *http.Server
}

// NewService creates a new API Gateway service
func NewService(cfg config.APIGatewayConfig, logger *logrus.Logger) (*Service, error) {
	// Connect to webhook registry
	webhookConn, err := grpc.Dial(cfg.WebhookRegistryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to webhook registry: %w", err)
	}

	// Connect to observability service
	obsConn, err := grpc.Dial(cfg.ObservabilityAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to observability service: %w", err)
	}

	// Initialize JWT manager
	jwtManager := security.NewJWTManager(
		"your-secret-key", // In production, get from config
		"webhook-service",
		"webhook-api",
		time.Hour*24,
	)

	// Initialize auth middleware
	authMiddleware := security.NewAuthMiddleware(jwtManager, nil, []string{"/health", "/auth/login"})

	service := &Service{
		config:              cfg,
		log:                 logger,
		webhookClient:       pb.NewWebhookRegistryClient(webhookConn),
		observabilityClient: pb.NewObservabilityServiceClient(obsConn),
		auth:                authMiddleware,
		jwtManager:          jwtManager,
	}

	return service, nil
}

// Start starts the API Gateway HTTP server
func (s *Service) Start() error {
	router := s.setupRoutes()

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		Handler:      router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	s.log.WithField("address", s.server.Addr).Info("Starting API Gateway HTTP server")

	return s.server.ListenAndServe()
}

// Stop stops the API Gateway HTTP server
func (s *Service) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// setupRoutes sets up HTTP routes
func (s *Service) setupRoutes() *mux.Router {
	router := mux.NewRouter()

	// Add logging middleware
	router.Use(s.loggingMiddleware)

	// Add CORS middleware if enabled
	if s.config.CORSEnabled {
		router.Use(s.corsMiddleware)
	}

	// Public routes
	router.HandleFunc("/health", s.healthHandler).Methods("GET")
	router.HandleFunc("/auth/login", s.loginHandler).Methods("POST")

	// Protected routes
	api := router.PathPrefix("/api/v1").Subrouter()
	api.Use(s.auth.Middleware)

	// Webhook management endpoints
	api.HandleFunc("/webhooks", s.createWebhookHandler).Methods("POST")
	api.HandleFunc("/webhooks", s.listWebhooksHandler).Methods("GET")
	api.HandleFunc("/webhooks/{id}", s.getWebhookHandler).Methods("GET")
	api.HandleFunc("/webhooks/{id}", s.updateWebhookHandler).Methods("PUT")
	api.HandleFunc("/webhooks/{id}", s.deleteWebhookHandler).Methods("DELETE")

	// Webhook logs and metrics endpoints
	api.HandleFunc("/webhooks/{id}/logs", s.getWebhookLogsHandler).Methods("GET")
	api.HandleFunc("/webhooks/{id}/stats", s.getWebhookStatsHandler).Methods("GET")

	return router
}

// HTTP Handlers

func (s *Service) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *Service) loginHandler(w http.ResponseWriter, r *http.Request) {
	// Simplified login - in production, validate credentials
	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken("customer-123", []string{"webhook:read", "webhook:write"})
	if err != nil {
		s.log.WithError(err).Error("Failed to generate token")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (s *Service) createWebhookHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL        string            `json:"url"`
		EventTypes []string          `json:"event_types"`
		Secret     string            `json:"secret,omitempty"`
		Headers    map[string]string `json:"headers,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get customer ID from JWT token
	customerID := s.getCustomerIDFromContext(r.Context())

	// Call webhook registry service
	grpcReq := &pb.CreateWebhookRequest{
		CustomerId: customerID,
		Url:        req.URL,
		EventTypes: req.EventTypes,
		Secret:     req.Secret,
		Headers:    req.Headers,
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.GRPCTimeout)
	defer cancel()

	resp, err := s.webhookClient.CreateWebhook(ctx, grpcReq)
	if err != nil {
		s.log.WithError(err).Error("Failed to create webhook")
		http.Error(w, "Failed to create webhook", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"webhook_id": resp.WebhookId,
		"secret":     resp.Secret,
		"created_at": resp.CreatedAt.AsTime(),
	})
}

func (s *Service) listWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	customerID := s.getCustomerIDFromContext(r.Context())

	// Parse query parameters
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	grpcReq := &pb.GetWebhooksByCustomerRequest{
		CustomerId: customerID,
		Limit:      int32(limit),
		Offset:     int32(offset),
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.GRPCTimeout)
	defer cancel()

	resp, err := s.webhookClient.GetWebhooksByCustomer(ctx, grpcReq)
	if err != nil {
		s.log.WithError(err).Error("Failed to list webhooks")
		http.Error(w, "Failed to list webhooks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"webhooks":    resp.Webhooks,
		"total_count": resp.TotalCount,
	})
}

func (s *Service) getWebhookHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	webhookID := vars["id"]
	customerID := s.getCustomerIDFromContext(r.Context())

	grpcReq := &pb.GetWebhookRequest{
		WebhookId:  webhookID,
		CustomerId: customerID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.GRPCTimeout)
	defer cancel()

	resp, err := s.webhookClient.GetWebhook(ctx, grpcReq)
	if err != nil {
		s.log.WithError(err).Error("Failed to get webhook")
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.Webhook)
}

func (s *Service) updateWebhookHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	webhookID := vars["id"]
	customerID := s.getCustomerIDFromContext(r.Context())

	var req struct {
		URL        *string           `json:"url,omitempty"`
		EventTypes []string          `json:"event_types,omitempty"`
		Headers    map[string]string `json:"headers,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	grpcReq := &pb.UpdateWebhookRequest{
		WebhookId:  webhookID,
		CustomerId: customerID,
		Url:        req.URL,
		EventTypes: req.EventTypes,
		Headers:    req.Headers,
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.GRPCTimeout)
	defer cancel()

	resp, err := s.webhookClient.UpdateWebhook(ctx, grpcReq)
	if err != nil {
		s.log.WithError(err).Error("Failed to update webhook")
		http.Error(w, "Failed to update webhook", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.Webhook)
}

func (s *Service) deleteWebhookHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	webhookID := vars["id"]
	customerID := s.getCustomerIDFromContext(r.Context())

	grpcReq := &pb.DeleteWebhookRequest{
		WebhookId:  webhookID,
		CustomerId: customerID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.GRPCTimeout)
	defer cancel()

	_, err := s.webhookClient.DeleteWebhook(ctx, grpcReq)
	if err != nil {
		s.log.WithError(err).Error("Failed to delete webhook")
		http.Error(w, "Failed to delete webhook", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) getWebhookLogsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	webhookID := vars["id"]

	// Parse query parameters
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	grpcReq := &pb.GetWebhookLogsRequest{
		WebhookId: webhookID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.GRPCTimeout)
	defer cancel()

	resp, err := s.observabilityClient.GetWebhookLogs(ctx, grpcReq)
	if err != nil {
		s.log.WithError(err).Error("Failed to get webhook logs")
		http.Error(w, "Failed to get webhook logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":        resp.Logs,
		"total_count": resp.TotalCount,
	})
}

func (s *Service) getWebhookStatsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	webhookID := vars["id"]

	grpcReq := &pb.GetDeliveryStatsRequest{
		WebhookId: webhookID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.GRPCTimeout)
	defer cancel()

	resp, err := s.observabilityClient.GetDeliveryStats(ctx, grpcReq)
	if err != nil {
		s.log.WithError(err).Error("Failed to get webhook stats")
		http.Error(w, "Failed to get webhook stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.Stats)
}

// Middleware

func (s *Service) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a wrapper to capture status code
		wrapper := &responseWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)

		s.log.WithFields(logrus.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status_code": wrapper.statusCode,
			"duration":    duration,
			"remote_addr": r.RemoteAddr,
		}).Info("HTTP request completed")
	})
}

func (s *Service) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Helper functions

func (s *Service) getCustomerIDFromContext(ctx context.Context) string {
	// In a real implementation, extract customer ID from JWT token in context
	return "customer-123"
}

// responseWrapper wraps http.ResponseWriter to capture status code
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
