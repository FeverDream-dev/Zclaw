package runtime

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
)

type WorkerType string

const (
	WorkerTool    WorkerType = "tool-worker"
	WorkerBrowser WorkerType = "browser-worker"
)

type WorkerConfig struct {
	Type         WorkerType        `json:"type"`
	Image        string            `json:"image"`
	AgentID      string            `json:"agent_id"`
	TaskID       string            `json:"task_id"`
	WorkspaceDir string            `json:"workspace_dir"`
	MemoryMB     int               `json:"memory_mb"`
	CPUFraction  float64           `json:"cpu_fraction"`
	Timeout      time.Duration     `json:"timeout"`
	Env          map[string]string `json:"env"`
	ReadOnlyFS   bool              `json:"read_only_fs"`
	NetworkMode  string            `json:"network_mode"`
}

type WorkerStatus struct {
	ContainerID string     `json:"container_id"`
	AgentID     string     `json:"agent_id"`
	TaskID      string     `json:"task_id"`
	Type        WorkerType `json:"type"`
	State       string     `json:"state"`
	StartedAt   time.Time  `json:"started_at"`
	ExitCode    int        `json:"exit_code,omitempty"`
}

type DockerAPIClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, name string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
}

type WorkerPoolImpl struct {
	client   DockerAPIClient
	poolSize int
	active   map[string]WorkerStatus
	mu       sync.Mutex
}

func NewWorkerPool(client DockerAPIClient, poolSize int) *WorkerPoolImpl {
	return &WorkerPoolImpl{
		client:   client,
		poolSize: poolSize,
		active:   make(map[string]WorkerStatus),
	}
}

func (p *WorkerPoolImpl) Launch(ctx context.Context, config WorkerConfig) (*WorkerStatus, error) {
	p.mu.Lock()
	if len(p.active) >= p.poolSize {
		p.mu.Unlock()
		return nil, fmt.Errorf("worker pool full (%d/%d)", len(p.active), p.poolSize)
	}
	p.mu.Unlock()

	containerName := fmt.Sprintf("zclaw-%s-%s", config.Type, config.TaskID)

	env := make([]string, 0, len(config.Env))
	for k, v := range config.Env {
		env = append(env, k+"="+v)
	}

	contConfig := &container.Config{
		Image: config.Image,
		Env:   env,
		Labels: map[string]string{
			"zclaw.managed":     "true",
			"zclaw.agent":       config.AgentID,
			"zclaw.task":        config.TaskID,
			"zclaw.worker.type": string(config.Type),
		},
	}

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:    int64(config.MemoryMB) * 1024 * 1024,
			CPUQuota:  int64(config.CPUFraction * 100000),
			CPUPeriod: 100000,
		},
		AutoRemove: true,
	}

	if config.WorkspaceDir != "" {
		hostConfig.Binds = []string{config.WorkspaceDir + ":/workspace:rw"}
	}

	resp, err := p.client.ContainerCreate(ctx, contConfig, hostConfig, containerName)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	if err := p.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		p.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("start container: %w", err)
	}

	status := &WorkerStatus{
		ContainerID: resp.ID,
		AgentID:     config.AgentID,
		TaskID:      config.TaskID,
		Type:        config.Type,
		State:       "running",
		StartedAt:   time.Now().UTC(),
	}

	p.mu.Lock()
	p.active[resp.ID] = *status
	p.mu.Unlock()

	return status, nil
}

func (p *WorkerPoolImpl) Stop(ctx context.Context, containerID string) error {
	err := p.client.ContainerStop(ctx, containerID, container.StopOptions{})
	p.mu.Lock()
	delete(p.active, containerID)
	p.mu.Unlock()
	return err
}

func (p *WorkerPoolImpl) Status(ctx context.Context, containerID string) (*WorkerStatus, error) {
	inspect, err := p.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("inspect container %s: %w", containerID, err)
	}
	return &WorkerStatus{
		ContainerID: containerID,
		State:       inspect.State.Status,
		ExitCode:    inspect.State.ExitCode,
	}, nil
}

func (p *WorkerPoolImpl) List(ctx context.Context) ([]WorkerStatus, error) {
	f := filters.NewArgs()
	f.Add("label", "zclaw.managed=true")
	containers, err := p.client.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		return nil, err
	}

	statuses := make([]WorkerStatus, 0, len(containers))
	for _, c := range containers {
		statuses = append(statuses, WorkerStatus{
			ContainerID: c.ID,
			AgentID:     c.Labels["zclaw.agent"],
			TaskID:      c.Labels["zclaw.task"],
			Type:        WorkerType(c.Labels["zclaw.worker.type"]),
			State:       c.State,
		})
	}
	return statuses, nil
}

func (p *WorkerPoolImpl) StreamLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	return p.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		Follow: true, ShowStdout: true, ShowStderr: true, Timestamps: true,
	})
}

func (p *WorkerPoolImpl) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().UTC().Add(-olderThan)
	f := filters.NewArgs()
	f.Add("label", "zclaw.managed=true")
	f.Add("status", "exited")
	f.Add("status", "dead")
	containers, err := p.client.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, c := range containers {
		if time.Unix(c.Created, 0).Before(cutoff) {
			_ = p.client.ContainerRemove(ctx, c.ID, container.RemoveOptions{})
			p.mu.Lock()
			delete(p.active, c.ID)
			p.mu.Unlock()
			removed++
		}
	}
	return removed, nil
}

func (p *WorkerPoolImpl) PoolSize() int    { return p.poolSize }
func (p *WorkerPoolImpl) ActiveCount() int { p.mu.Lock(); defer p.mu.Unlock(); return len(p.active) }
