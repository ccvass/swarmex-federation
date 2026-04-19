package federation

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

const (
	labelReplicate = "swarmex.federation.replicate"
	labelClusters  = "swarmex.federation.clusters"
)

type Controller struct {
	local    *client.Client
	remotes  map[string]*client.Client
	logger   *slog.Logger
	pending  map[string]bool
	mu       sync.Mutex
}

func New(local *client.Client, remoteAddrs map[string]string, logger *slog.Logger) *Controller {
	c := &Controller{local: local, remotes: make(map[string]*client.Client), logger: logger, pending: make(map[string]bool)}
	for name, addr := range remoteAddrs {
		cli, err := client.NewClientWithOpts(client.WithHost(addr), client.WithAPIVersionNegotiation())
		if err != nil {
			logger.Error("failed to connect to remote cluster", "cluster", name, "addr", addr, "error", err)
			continue
		}
		c.remotes[name] = cli
		logger.Info("connected to remote cluster", "cluster", name, "addr", addr)
	}
	return c
}

func (c *Controller) HandleEvent(ctx context.Context, event events.Message) {
	if event.Type != events.ServiceEventType { return }
	if event.Action != events.ActionCreate && event.Action != events.ActionUpdate { return }
	c.mu.Lock()
	if c.pending[event.Actor.ID] { c.mu.Unlock(); return }
	c.pending[event.Actor.ID] = true
	c.mu.Unlock()
	go func() {
		time.Sleep(3 * time.Second)
		c.reconcile(ctx, event.Actor.ID)
		c.mu.Lock()
		delete(c.pending, event.Actor.ID)
		c.mu.Unlock()
	}()
}

func (c *Controller) reconcile(ctx context.Context, serviceID string) {
	svc, _, err := c.local.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err != nil { return }

	if svc.Spec.Labels[labelReplicate] != "true" { return }

	targets := strings.Split(svc.Spec.Labels[labelClusters], ",")
	for _, target := range targets {
		target = strings.TrimSpace(target)
		remote, ok := c.remotes[target]
		if !ok {
			c.logger.Warn("unknown remote cluster", "cluster", target, "service", svc.Spec.Name)
			continue
		}
		c.replicateService(ctx, remote, svc, target)
	}
}

func (c *Controller) replicateService(ctx context.Context, remote *client.Client, svc swarm.Service, clusterName string) {
	// Check if service already exists on remote
	_, _, err := remote.ServiceInspectWithRaw(ctx, svc.Spec.Name, types.ServiceInspectOptions{})
	if err == nil {
		// Exists — update
		remoteSvc, _, _ := remote.ServiceInspectWithRaw(ctx, svc.Spec.Name, types.ServiceInspectOptions{})
		remoteSvc.Spec.TaskTemplate = svc.Spec.TaskTemplate
		remoteSvc.Spec.Labels = svc.Spec.Labels
		_, err = remote.ServiceUpdate(ctx, remoteSvc.ID, remoteSvc.Version, remoteSvc.Spec, types.ServiceUpdateOptions{})
		if err != nil {
			c.logger.Error("federation update failed", "service", svc.Spec.Name, "cluster", clusterName, "error", err)
			return
		}
		c.logger.Info("federation updated", "service", svc.Spec.Name, "cluster", clusterName)
		return
	}

	// Create on remote
	_, err = remote.ServiceCreate(ctx, svc.Spec, types.ServiceCreateOptions{})
	if err != nil {
		c.logger.Error("federation create failed", "service", svc.Spec.Name, "cluster", clusterName, "error", err)
		return
	}
	c.logger.Info("federation replicated", "service", svc.Spec.Name, "cluster", clusterName)
}
