package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type WebhookChannel struct {
	*BaseChannel
	config     config.WebhookConfig
	httpServer *http.Server
	mu         sync.Mutex
	running    bool
}

// WebhookRequest is the payload format expected from n8n/Evolution
type WebhookRequest struct {
	SenderID string            `json:"sender_id"`
	ChatID   string            `json:"chat_id"`
	Content  string            `json:"content"`
	Media    []string          `json:"media,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// WebhookResponse is the format sent back to n8n
type WebhookResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func NewWebhookChannel(cfg config.WebhookConfig, messageBus *bus.MessageBus) (*WebhookChannel, error) {
	base := NewBaseChannel("webhook", cfg, messageBus, cfg.AllowFrom)

	return &WebhookChannel{
		BaseChannel: base,
		config:      cfg,
		running:     false,
	}, nil
}

func (c *WebhookChannel) Start(ctx context.Context) error {
	logger.InfoCF("channels", "Starting Webhook channel", map[string]interface{}{
		"port": c.config.Port,
		"path": c.config.Path,
	})

	mux := http.NewServeMux()
	mux.HandleFunc(c.config.Path, c.handleWebhook)

	addr := fmt.Sprintf("0.0.0.0:%d", c.config.Port)
	c.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	c.mu.Lock()
	c.running = true
	c.mu.Unlock()

	go func() {
		if err := c.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.ErrorCF("channels", "Webhook server error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	logger.InfoC("channels", "Webhook channel started")
	return nil
}

func (c *WebhookChannel) Stop(ctx context.Context) error {
	logger.InfoC("channels", "Stopping Webhook channel")

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.httpServer != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := c.httpServer.Shutdown(ctx); err != nil {
			logger.ErrorCF("channels", "Error shutting down webhook server", map[string]interface{}{
				"error": err.Error(),
			})
		}
		c.httpServer = nil
	}

	c.running = false
	return nil
}

func (c *WebhookChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	// For now, webhooks are mostly inbound.
	// Outbound messages could be sent to a callback URL if configured,
	// but typically n8n waits for the HTTP response or we use another webhook.
	
	// If CallbackURL is configured, send the response there
	if c.config.CallbackURL != "" {
		payload := map[string]interface{}{
			"chat_id": msg.ChatID,
			"content": msg.Content,
			"type":    "text",
		}
		
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		
		resp, err := http.Post(c.config.CallbackURL, "application/json", bytes.NewBuffer(data))
		if err != nil {
			return fmt.Errorf("failed to send callback: %w", err)
		}
		defer resp.Body.Close()
		
		return nil
	}
	
	logger.InfoCF("channels", "Webhook outbound message (no callback configured)", map[string]interface{}{
		"chat_id": msg.ChatID,
		"content": msg.Content,
	})
	
	return nil
}

func (c *WebhookChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Basic Auth or Token check
	if c.config.SecretToken != "" {
		token := r.Header.Get("X-Webhook-Token")
		if token != c.config.SecretToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	var req WebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	logger.InfoCF("channels", "Received webhook message", map[string]interface{}{
		"sender": req.SenderID,
		"chat":   req.ChatID,
	})

	// Process message
	c.HandleMessage(req.SenderID, req.ChatID, req.Content, req.Media, req.Metadata)

	// Send immediate response to acknowledge receipt
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(WebhookResponse{
		Status:  "ok",
		Message: "Message received",
	})
}
