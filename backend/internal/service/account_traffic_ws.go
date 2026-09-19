package service

import (
	"context"
	"sync"

	openaiwsv2 "github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2"
	coderws "github.com/coder/websocket"
	"github.com/tidwall/gjson"
)

// Passthrough may carry many independent response.create turns. Count each
// turn, release at its terminal event, and do not charge session.update/pings.
type accountTrafficFrameConn struct {
	inner   openaiwsv2.FrameConn
	control *AccountTrafficService
	plan    AccountTrafficPlan
	ctx     context.Context
	mu      sync.Mutex
	pending map[string][]*AccountTrafficPermit
}

// A queue per stream preserves FIFO completion, including overlapping creates.
// Observation alone must never impose an extra concurrency limit.
func (c *accountTrafficFrameConn) finish(status int) {
	c.mu.Lock()
	pending := c.pending
	c.pending = nil
	c.mu.Unlock()
	for _, permits := range pending {
		for _, p := range permits {
			p.Finish(status)
		}
	}
}
func (c *accountTrafficFrameConn) finishStream(stream string, status int) {
	c.mu.Lock()
	var p *AccountTrafficPermit
	if pending := c.pending[stream]; len(pending) > 0 {
		p = pending[0]
		if len(pending) == 1 {
			delete(c.pending, stream)
		} else {
			c.pending[stream] = pending[1:]
		}
	}
	c.mu.Unlock()
	p.Finish(status)
}
func (c *accountTrafficFrameConn) ReadFrame(ctx context.Context) (coderws.MessageType, []byte, error) {
	kind, payload, err := c.inner.ReadFrame(ctx)
	if err != nil {
		c.finish(0)
		return kind, payload, err
	}
	if status, terminal := accountTrafficEventStatus(payload); terminal {
		c.finishStream(gjson.GetBytes(payload, "stream_id").String(), status)
	}
	return kind, payload, nil
}
func (c *accountTrafficFrameConn) WriteFrame(ctx context.Context, kind coderws.MessageType, payload []byte) error {
	if gjson.GetBytes(payload, "type").String() != "response.create" {
		return c.inner.WriteFrame(ctx, kind, payload)
	}
	_, p, err := c.control.Begin(c.ctx, c.plan, func() { _ = c.inner.Close() })
	if err != nil {
		if failure := AccountTrafficFailover(err); failure != nil {
			return failure
		}
		return err
	}
	stream := gjson.GetBytes(payload, "stream_id").String()
	c.mu.Lock()
	if c.pending == nil {
		c.pending = make(map[string][]*AccountTrafficPermit)
	}
	c.pending[stream] = append(c.pending[stream], p)
	c.mu.Unlock()
	err = c.inner.WriteFrame(ctx, kind, payload)
	if err != nil {
		c.finish(0)
	}
	return err
}
func (c *accountTrafficFrameConn) Close() error { c.finish(0); return c.inner.Close() }
func wrapAccountTrafficFrameConn(ctx context.Context, upstream HTTPUpstream, a *Account, inner openaiwsv2.FrameConn) (openaiwsv2.FrameConn, error) {
	plan, err := AccountTrafficPlanFor(a)
	if err != nil {
		return nil, err
	}
	return &accountTrafficFrameConn{inner: inner, control: accountTrafficController(upstream), plan: plan, ctx: ctx}, nil
}
