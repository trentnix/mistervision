package companion

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"
)

// authenticate caches successful viewer checks for one minute. The cache is
// bounded and discarded when the source stops. Raw credentials are never retained.
func (c *receiver) authenticate(ctx context.Context, client, peer, token string) (*controller, error) {
	if client == "" || len(client) > 128 || strings.ContainsAny(client, "\r\n\x00") || token == "" || len(token) > 4096 {
		return nil, errors.New(messageUnauthorized)
	}
	key := controllerKey{client: client, peer: peer, credential: sha256.Sum256([]byte(token))}
	now := time.Now()
	c.mu.Lock()
	entry := c.controllers[key]
	if entry != nil && now.Sub(entry.verified) < time.Minute {
		entry.seen = now
		c.mu.Unlock()
		return entry, nil
	}
	c.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := c.config.Authorize(ctx, token); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx == nil || c.ctx.Err() != nil {
		return nil, errors.New(messageUnavailable)
	}
	for key, entry := range c.controllers {
		if now.Sub(entry.seen) > 2*time.Minute {
			delete(c.controllers, key)
		}
	}
	entry = c.controllers[key]
	if entry == nil {
		if len(c.controllers) >= 32 {
			return nil, errors.New(messageBusy)
		}
		entry = &controller{}
		c.controllers[key] = entry
	}
	entry.verified, entry.seen = now, now
	return entry, nil
}
