package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	cliflag "k8s.io/component-base/cli/flag"

	"quortex.io/node-drainer/pkg/drainer"
	"quortex.io/node-drainer/pkg/logger"
)

func main() {
	var (
		fEnableDevLogs         bool
		fLogVerbosity          int
		fSelector              map[string]string
		fEvictionGlobalTimeout int
		fOlderThan             time.Duration
		fCount                 int
		fMaxUnscheduledPods    int
		fKubeConfig            string
	)

	flag.BoolVar(&fEnableDevLogs, "dev", false, "Enable dev mode for logging.")
	flag.IntVar(&fLogVerbosity, "v", 3, "Logs verbosity. 0 => panic, 1 => error, 2 => warning, 3 => info, 4 => debug")
	flag.Var(cliflag.NewMapStringString(&fSelector), "l", "Selector to list the nodes to drain on labels separated by commas (e.g. `-l foo=bar,bar=baz`).")
	flag.IntVar(&fEvictionGlobalTimeout, "eviction-timeout", 300, "The timeout in seconds for pods eviction during node drain.")
	flag.DurationVar(&fOlderThan, "older-than", time.Hour*8, "The minimum lifespan that a node must have to be drained.")
	flag.IntVar(&fCount, "count", 1, "The number of nodes to drain.")
	flag.IntVar(&fMaxUnscheduledPods, "max-unscheduled-pods", 0, "The maximum number of unscheduled pods on the cluster beyond which the drain will fail.")
	flag.StringVar(&fKubeConfig, "kubeconfig", "", "(optional) absolute path to the kubeconfig file")
	flag.Parse()

	// Init logger
	log := logger.NewLogger(logger.Config{Dev: fEnableDevLogs, Verbosity: fLogVerbosity})

	var config *rest.Config
	var err error

	if fKubeConfig != "" {
		// KubeConfig was set, use the provided kubeconfig
		log.Info("Try to configure k8s client from provided kubeconfig")
		config, err = clientcmd.BuildConfigFromFlags("", fKubeConfig)
	} else {
		// Else try to use kubernetes configuration
		config, err = rest.InClusterConfig()
		if homedir.HomeDir() != "" && errors.Is(err, rest.ErrNotInCluster) {
			// Else if we are in a homedir, try to use ~/.kube/config
			config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
		}
	}
	if err != nil {
		log.Error(err, "Failed to init kubernetes rest config")
		os.Exit(1)
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create Clientset for the given config")
		os.Exit(1)
	}

	// Perform node drains
	d := drainer.New(drainer.Configuration{
		EvictionGlobalTimeout: fEvictionGlobalTimeout,
		Cli:                   clientset,
		Log:                   log,
	})
	if err := d.Drain(context.Background(), fSelector, fOlderThan, fCount, fMaxUnscheduledPods); err != nil {
		log.Error(err, "Failed to drain nodes")
		os.Exit(1)
	}
}
