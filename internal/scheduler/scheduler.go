package scheduler

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/zclaw/zclaw/internal/agents"
	"github.com/zclaw/zclaw/internal/providers"
)

type TaskID string

func (id TaskID) String() string { return string(id) }

type TaskState string

const (
	TaskPending   TaskState = "pending"
	TaskRunning   TaskState = "running"
	TaskCompleted TaskState = "completed"
	TaskFailed    TaskState = "failed"
	TaskRetrying  TaskState = "retrying"
	TaskCancelled TaskState = "cancelled"
)

func (s TaskState) String() string { return string(s) }

type TaskPriority int

const (
	PriorityLow      TaskPriority = 0
	PriorityNormal   TaskPriority = 5
	PriorityHigh     TaskPriority = 10
	PriorityCritical TaskPriority = 20
)

type Task struct {
	ID             TaskID         `json:"id"`
	AgentID        agents.AgentID `json:"agent_id"`
	State          TaskState      `json:"state"`
	Priority       TaskPriority   `json:"priority"`
	Attempt        int            `json:"attempt"`
	MaxAttempts    int            `json:"max_attempts"`
	Input          string         `json:"input"`
	Output         string         `json:"output,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	ModelUsed      string         `json:"model_used,omitempty"`
	ProviderID     string         `json:"provider_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	StartedAt      *time.Time     `json:"started_at,omitempty"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
	ScheduledAt    *time.Time     `json:"scheduled_at,omitempty"`
	TimeoutSeconds int            `json:"timeout_seconds"`
}

type TaskResult struct {
	TaskID    TaskID          `json:"task_id"`
	Output    string          `json:"output"`
	Usage     providers.Usage `json:"usage"`
	ModelUsed string          `json:"model_used"`
	Error     error           `json:"-"`
}

type TaskHandler func(ctx context.Context, task Task, agent agents.Agent) (*TaskResult, error)

type EnqueueOptions struct {
	AgentID        agents.AgentID
	Input          string
	Priority       TaskPriority
	ScheduledAt    *time.Time
	MaxAttempts    int
	TimeoutSeconds int
}

type SchedulerConfig struct {
	MaxWorkers        int           `json:"max_workers"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	JitterSeconds     int           `json:"jitter_seconds"`
	SweepInterval     time.Duration `json:"sweep_interval"`
	RetryBackoffBase  time.Duration `json:"retry_backoff_base"`
	RetryBackoffMax   time.Duration `json:"retry_backoff_max"`
}

func DefaultConfig() SchedulerConfig {
	return SchedulerConfig{
		MaxWorkers:        10,
		HeartbeatInterval: 60 * time.Second,
		JitterSeconds:     30,
		SweepInterval:     5 * time.Minute,
		RetryBackoffBase:  30 * time.Second,
		RetryBackoffMax:   30 * time.Minute,
	}
}

type event struct {
	kind    string
	agentID agents.AgentID
	taskID  TaskID
}

type Scheduler struct {
	config  SchedulerConfig
	agents  agents.Registry
	handler TaskHandler

	mu      sync.Mutex
	queue   []Task
	running map[TaskID]Task
	events  chan event

	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewScheduler(cfg SchedulerConfig, agentReg agents.Registry, handler TaskHandler) *Scheduler {
	return &Scheduler{
		config:  cfg,
		agents:  agentReg,
		handler: handler,
		running: make(map[TaskID]Task),
		events:  make(chan event, 256),
		stopCh:  make(chan struct{}),
	}
}

func (s *Scheduler) Enqueue(ctx context.Context, opts EnqueueOptions) (*Task, error) {
	task := Task{
		ID:             TaskID(fmt.Sprintf("%x", rand.Int64())),
		AgentID:        opts.AgentID,
		State:          TaskPending,
		Priority:       opts.Priority,
		MaxAttempts:    opts.MaxAttempts,
		Input:          opts.Input,
		ScheduledAt:    opts.ScheduledAt,
		TimeoutSeconds: opts.TimeoutSeconds,
		CreatedAt:      time.Now().UTC(),
	}
	if task.MaxAttempts <= 0 {
		task.MaxAttempts = 3
	}
	if task.TimeoutSeconds <= 0 {
		task.TimeoutSeconds = 300
	}

	s.mu.Lock()
	s.queue = append(s.queue, task)
	s.mu.Unlock()

	s.events <- event{kind: "task_enqueued", agentID: opts.AgentID, taskID: task.ID}
	return &task, nil
}

func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.runEventLoop(ctx)
	s.wg.Add(1)
	go s.runSweepLoop(ctx)
	s.wg.Add(1)
	go s.runWorkerPool(ctx)
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *Scheduler) runEventLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-s.stopCh:
			return
		case evt := <-s.events:
			if evt.kind == "task_completed" || evt.kind == "task_failed" {
				s.mu.Lock()
				delete(s.running, evt.taskID)
				s.mu.Unlock()
				s.events <- event{kind: "agent_wake", agentID: evt.agentID}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) runSweepLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.config.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			scheduled, err := s.agents.GetBySchedule(ctx, time.Now().UTC())
			if err != nil {
				continue
			}
			for _, agent := range scheduled {
				jitter := time.Duration(rand.IntN(s.config.JitterSeconds)) * time.Second
				go func(a agents.Agent, j time.Duration) {
					time.Sleep(j)
					s.events <- event{kind: "agent_wake", agentID: a.ID}
				}(agent, jitter)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) runWorkerPool(ctx context.Context) {
	defer s.wg.Done()
	sem := make(chan struct{}, s.config.MaxWorkers)
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		task := s.dequeueNext()
		if task == nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		sem <- struct{}{}
		s.mu.Lock()
		s.running[task.ID] = *task
		s.mu.Unlock()

		s.wg.Add(1)
		go func(t Task) {
			defer s.wg.Done()
			defer func() { <-sem }()
			s.executeTask(ctx, t)
		}(*task)
	}
}

func (s *Scheduler) dequeueNext() *Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		return nil
	}

	best := 0
	for i, t := range s.queue {
		if t.Priority > s.queue[best].Priority {
			best = i
		}
	}

	task := s.queue[best]
	s.queue = append(s.queue[:best], s.queue[best+1:]...)
	return &task
}

func (s *Scheduler) executeTask(ctx context.Context, task Task) {
	agent, err := s.agents.Get(ctx, task.AgentID)
	if err != nil {
		s.events <- event{kind: "task_failed", agentID: task.AgentID, taskID: task.ID}
		return
	}

	taskCtx, cancel := context.WithTimeout(ctx, time.Duration(task.TimeoutSeconds)*time.Second)
	defer cancel()

	now := time.Now().UTC()
	task.StartedAt = &now
	task.Attempt++

	result, err := s.handler(taskCtx, task, *agent)
	if err != nil {
		task.State = TaskFailed
		task.ErrorMessage = err.Error()
		if task.Attempt < task.MaxAttempts {
			task.State = TaskRetrying
			backoff := s.config.RetryBackoffBase * time.Duration(1<<(task.Attempt-1))
			if backoff > s.config.RetryBackoffMax {
				backoff = s.config.RetryBackoffMax
			}
			future := time.Now().UTC().Add(backoff)
			task.ScheduledAt = &future
			s.mu.Lock()
			s.queue = append(s.queue, task)
			s.mu.Unlock()
		}
		s.events <- event{kind: "task_failed", agentID: task.AgentID, taskID: task.ID}
		return
	}

	task.State = TaskCompleted
	task.Output = result.Output
	task.ModelUsed = result.ModelUsed
	completed := time.Now().UTC()
	task.CompletedAt = &completed

	s.events <- event{kind: "task_completed", agentID: task.AgentID, taskID: task.ID}
}

func (s *Scheduler) QueueDepth() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}

func (s *Scheduler) ActiveWorkers() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.running)
}
