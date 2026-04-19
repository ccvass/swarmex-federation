package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	federation "github.com/ccvass/swarmex/swarmex-federation"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil { logger.Error("docker failed", "error", err); os.Exit(1) }
	defer cli.Close()
	remotes := make(map[string]string)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "FEDERATION_CLUSTER_") {
			parts := strings.SplitN(env, "=", 2)
			name := strings.TrimPrefix(parts[0], "FEDERATION_CLUSTER_")
			remotes[strings.ToLower(name)] = parts[1]
		}
	}
	ctrl := federation.New(cli, remotes, logger)
	go func() { http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }); http.ListenAndServe(":8080", nil) }()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	logger.Info("swarmex-federation starting", "remotes", len(remotes))
	msgCh, errCh := cli.Events(ctx, events.ListOptions{})
	for { select { case e := <-msgCh: ctrl.HandleEvent(ctx, e); case err := <-errCh: if ctx.Err() != nil { return }; logger.Error("error", "err", err); return; case <-ctx.Done(): return } }
}
