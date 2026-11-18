// Command cron-scale-operator runs the CronScale controller manager.
package main

import (
	"flag"
	"os"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	platformv1 "github.com/bjo3g15nw7/cron-scale-operator/api/v1alpha1"
	"github.com/bjo3g15nw7/cron-scale-operator/internal/controller"
	"github.com/bjo3g15nw7/cron-scale-operator/internal/events"
)

var scheme = clientgoscheme.Scheme

func main() {
	var (
		metricsAddr = flag.String("metrics-addr", ":8080", "address the metrics endpoint binds to")
		probeAddr   = flag.String("health-probe-addr", ":8081", "address the probes bind to")
		leaderElect = flag.Bool("leader-elect", false, "enable leader election")
	)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	if err := platformv1.AddToScheme(scheme); err != nil {
		ctrl.Log.Error(err, "registering CronScale types")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: *metricsAddr},
		HealthProbeBindAddress: *probeAddr,
		LeaderElection:         *leaderElect,
		LeaderElectionID:       "cron-scale-operator.platform.milan.dev",
	})
	if err != nil {
		ctrl.Log.Error(err, "building manager")
		os.Exit(1)
	}

	if err := (&controller.CronScaleReconciler{
		Client: mgr.GetClient(),
		Events: events.New(mgr.GetEventRecorderFor("cron-scale-operator")),
	}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "wiring CronScaleReconciler")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "adding health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		ctrl.Log.Error(err, "adding readiness check")
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "running manager")
		os.Exit(1)
	}
}
