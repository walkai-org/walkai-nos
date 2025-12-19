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
	"flag"
	"os"
	"time"

	"github.com/nebuly-ai/nos/internal/controllers/gpupartitioner"
	partitionermig "github.com/nebuly-ai/nos/internal/partitioning/mig"
	configv1alpha1 "github.com/nebuly-ai/nos/pkg/api/nos.nebuly.com/config/v1alpha1"
	"github.com/nebuly-ai/nos/pkg/api/nos.nebuly.com/v1alpha1"
	"github.com/nebuly-ai/nos/pkg/constant"
	gpumig "github.com/nebuly-ai/nos/pkg/gpu/mig"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const defaultRequeueIntervalSeconds = 10

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(configv1alpha1.AddToScheme(scheme))
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "",
		"The controller will load its initial configuration from this file. "+
			"Omit this flag to use the default configuration values. "+
			"Command-line flags override configuration from this file.")
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	options := ctrl.Options{Scheme: scheme}
	config := configv1alpha1.GpuPartitionerConfig{
		RequeueIntervalSeconds: defaultRequeueIntervalSeconds,
	}
	if configFile != "" {
		var err error
		options, err = options.AndFrom(ctrl.ConfigFile().AtPath(configFile).OfKind(&config))
		if err != nil {
			setupLog.Error(err, "unable to load the config file")
			os.Exit(1)
		}
	}

	if config.RequeueIntervalSeconds == 0 {
		config.RequeueIntervalSeconds = defaultRequeueIntervalSeconds
	}

	requeueInterval := config.RequeueIntervalSeconds * time.Second

	if config.KnownMigGeometriesFile != "" {
		knownGeometries, err := loadKnownMigGeometriesFromFile(config.KnownMigGeometriesFile)
		if err != nil {
			setupLog.Error(err, "unable to load known MIG geometries")
			os.Exit(1)
		}
		if err = gpumig.SetKnownGeometries(knownGeometries.GroupByModel()); err != nil {
			setupLog.Error(err, "unable to set known MIG geometries")
			os.Exit(1)
		}
		setupLog.Info("using known MIG geometries loaded from file", "geometries", knownGeometries)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to build kubernetes clientset")
		os.Exit(1)
	}

	nodeController := gpupartitioner.NewNodeController(
		mgr.GetClient(),
		mgr.GetScheme(),
		partitionermig.NewNodeInitializer(mgr.GetClient()),
	)
	if err = nodeController.SetupWithManager(mgr, constant.ClusterStateNodeControllerName); err != nil {
		setupLog.Error(err, "unable to create node controller")
		os.Exit(1)
	}

	migController := gpupartitioner.NewController(mgr.GetClient(), kubeClient, mgr.GetScheme(), requeueInterval, config.PreemptionEnabled)
	if err = migController.SetupWithManager(mgr, constant.MigPartitionerControllerName); err != nil {
		setupLog.Error(err, "unable to create MIG controller")
		os.Exit(1)
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func loadKnownMigGeometriesFromFile(file string) (gpumig.AllowedMigGeometriesList, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var allowedGeometries = make(gpumig.AllowedMigGeometriesList, 0)
	if err = yaml.Unmarshal(data, &allowedGeometries); err != nil {
		return nil, err
	}
	return allowedGeometries, nil
}
