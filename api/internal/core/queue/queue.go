// Package queue provides the Agent Action Queue abstraction.
//
// In the MVP Base Core, this is a lightweight in-memory queue.
// The reply-orchestrator module uses it to enforce ordered Agent replies.
// Future modules (Bounty review, narrative voting, etc.) reuse the same interface.
//
// For production: replace the in-memory implementation with Asynq + Redis
// without changing any module code — just swap this package's internals.
package queue

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task represents a unit of work queued for an Agent.
type Task struct {
	ID        string
	Type      string
	Payload   map[string]interface{}
	CreatedAt time.Time
}

// Result is returned by an Agent after completing a task.
type Result struct {
	TaskID  string
	AgentID string
	Output  map[string]interface{}
	Error   string
}

// AgentActionQueue is the core queue interface.
// Extension modules accept this interface so they can be tested with a stub.
type AgentActionQueue interface {
	// Enqueue adds a task and returns a queue token.
	Enqueue(ctx context.Context, task Task) (token string, err error)
	// Claim blocks until the next task is available for the given agent.
	Claim(ctx context.Context, agentID string) (*Task, error)
	// Submit records the agent's result for a claimed task.
	Submit(ctx context.Context, token string, result Result) error
	// Peek returns the current queue depth without claiming.
	Peek(ctx context.Context) int
}

// InMemoryQueue is the MVP implementation — suitable for single-instance deployments.
type InMemoryQueue struct {
	mu      sync.Mutex
	pending []Task
	claims  map[string]Task // token → task
}

var Global AgentActionQueue = &InMemoryQueue{
	claims: make(map[string]Task),
}

func (q *InMemoryQueue) Enqueue(_ context.Context, task Task) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	token := fmt.Sprintf("%s-%d", task.ID, time.Now().UnixNano())
	q.pending = append(q.pending, task)
	return token, nil
}

func (q *InMemoryQueue) Claim(ctx context.Context, agentID string) (*Task, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			q.mu.Lock()
			if len(q.pending) > 0 {
				task := q.pending[0]
				q.pending = q.pending[1:]
				q.mu.Unlock()
				return &task, nil
			}
			q.mu.Unlock()
		}
	}
}

func (q *InMemoryQueue) Submit(_ context.Context, token string, result Result) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.claims, token)
	return nil
}

func (q *InMemoryQueue) Peek(_ context.Context) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}
