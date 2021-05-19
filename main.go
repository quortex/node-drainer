package main

import (
	"context"
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

var fEnableDevLogs bool
var fLogVerbosity int
var fInCluster bool
var fSelector map[string]string
var fEvictionGlobalTimeout int
var fOlderThan time.Duration
var fCount int

func main() {

	flag.BoolVar(&fEnableDevLogs, "dev", false, "Enable dev mode for logging.")
	flag.IntVar(&fLogVerbosity, "v", 3, "Logs verbosity. 0 => panic, 1 => error, 2 => warning, 3 => info, 4 => debug")
	flag.BoolVar(&fInCluster, "incluster", true, "Wether this application runs in a Pod in the cluster.")
	flag.Var(cliflag.NewMapStringString(&fSelector), "l", "Selector to list the nodes to drain.")
	flag.IntVar(&fEvictionGlobalTimeout, "eviction-timeout", 300, "The timeout in seconds for pods eviction on Instance deletion.")
	flag.DurationVar(&fOlderThan, "older-than", time.Hour*8, "The minimum lifespan that a node must have to be drained.")
	flag.IntVar(&fCount, "count", 1, "The number of nodes to drain.")
	flag.Parse()

	// Init logger
	log := logger.NewLogger(logger.Config{Dev: fEnableDevLogs, Verbosity: fLogVerbosity})

	var config *rest.Config
	var err error
	if fInCluster {
		// InClusterConfig returns a config object which uses the service account
		// kubernetes gives to pods. It's intended for clients that expect to be
		// running inside a pod running on kubernetes. It will return ErrNotInCluster
		// if called from a process not running in a kubernetes environment.
		config, err = rest.InClusterConfig()
	} else {
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()
		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
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
	if err := d.Drain(context.Background(), fSelector, fOlderThan, fCount); err != nil {
		log.Error(err, "Failed to drain nodes")
		os.Exit(1)
	}
}
