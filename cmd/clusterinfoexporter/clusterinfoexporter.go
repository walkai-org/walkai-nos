/*
 * Copyright 2026 nebuly.com
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/*
 * Copyright 2023 nebuly.com.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/nebuly-ai/nos/pkg/clusterinfo"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	var endpoint string
	var interval time.Duration
	var httpTimeout time.Duration
	var apiToken string

	flag.StringVar(&endpoint, "endpoint", "", "HTTP endpoint to which the cluster snapshots are sent.")
	flag.DurationVar(&interval, "interval", time.Minute, "Interval between cluster snapshots (e.g. 30s, 5m).")
	flag.DurationVar(&httpTimeout, "http-timeout", 10*time.Second, "HTTP client timeout.")
	flag.StringVar(&apiToken, "api-token", "", "Token to authenticate against the cluster info endpoint.")
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	ctx := ctrl.SetupSignalHandler()
	logger := log.FromContext(ctx)

	if endpoint == "" {
		logger.Error(fmt.Errorf("missing endpoint"), "the --endpoint flag is required")
		os.Exit(1)
	}
	if interval <= 0 {
		logger.Info("interval must be positive, defaulting to 1m")
		interval = time.Minute
	}

	cfg := ctrl.GetConfigOrDie()
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "unable to create Kubernetes client")
		os.Exit(1)
	}

	collector := clusterinfo.NewCollector(kubeClient)
	httpClient := &http.Client{Timeout: httpTimeout}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("starting cluster info exporter", "endpoint", endpoint, "interval", interval)

	if err := sendSnapshot(ctx, collector, httpClient, endpoint, apiToken, logger); err != nil {
		logger.Error(err, "failed to send cluster snapshot")
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down cluster info exporter")
			return
		case <-ticker.C:
			if err := sendSnapshot(ctx, collector, httpClient, endpoint, apiToken, logger); err != nil {
				logger.Error(err, "failed to send cluster snapshot")
			}
		}
	}
}

func sendSnapshot(
	ctx context.Context,
	collector clusterinfo.Collector,
	client *http.Client,
	endpoint string,
	apiToken string,
	logger logr.Logger,
) error {
	snapshot, err := collector.Collect(ctx)
	if err != nil {
		return fmt.Errorf("collecting snapshot: %w", err)
	}
	body, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshalling snapshot: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				logger.Error(readErr, "failed to read response body", "statusCode", resp.StatusCode)
			} else if len(bodyBytes) > 0 {
				logger.Info("received error response body", "statusCode", resp.StatusCode, "body", string(bodyBytes))
			}
		}
		return fmt.Errorf("unexpected status code %s", resp.Status)
	}

	logger.Info("cluster snapshot sent", "endpoint", endpoint, "statusCode", resp.StatusCode)
	return nil
}
