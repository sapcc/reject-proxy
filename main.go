package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"github.com/elazarl/goproxy"
	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	api "k8s.io/kubernetes/pkg/apis/core"
)

var (
	proxy          *goproxy.ProxyHttpServer
	config         Config
	verbose        bool
	listenAddr     string
	certDir        string
	kubeconfigFile string
	kubeContext    string
	namespaceName  string
	configMapName  string
	clientset      *kubernetes.Clientset
)

type Config struct {
	BlockedUrls []struct {
		Name   string `yaml:"name"`
		Method string `yaml:"method"`
		URL    string `yaml:"url"`
	} `yaml:"blockedUrls"`
}

func init() {
	flag.BoolVar(&verbose, "v", false, "Enable verbose output")
	flag.StringVar(&listenAddr, "addr", ":8080", "Proxy listen address")
	flag.StringVar(&certDir, "cert-dir", "certs", "Path to store CA key and certificate in")
	flag.StringVar(&kubeconfigFile, "kubeconfig", "", "Use explicit kubeconfig file")
	flag.StringVar(&kubeContext, "context", "", "Use context")
	flag.StringVar(&namespaceName, "namespace", "default", "Namespace of config map")
	flag.StringVar(&configMapName, "config-map", "reject-proxy", "Name of config map where blocked URLs are stored")
}

func main() {
	flag.Parse()

	if len(flag.Args()) > 0 && flag.Args()[0] == "generate-ca" {
		err := createCA()
		if err != nil {
			log.Fatalf("Could not generate CA: %s", err)
		}

		os.Exit(0)
	}

	caCert, caKey, err := loadCA()
	if err != nil {
		log.Fatalf("Could not load CA: %s", err)
	}

	kubeconfig, err := kubeConfig(kubeconfigFile, kubeContext)
	if err != nil {
		log.Fatal("Failed to create kubeconfig", err)
	}

	clientset, err = kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		log.Fatal("Could not create client set", err)
	}

	config = Config{}
	sharedInformers := informers.NewFilteredSharedInformerFactory(clientset, time.Hour*1, namespaceName, informerListOpts)
	configmapInformer := sharedInformers.Core().V1().ConfigMaps().Informer()
	stopper := make(chan struct{})
	defer close(stopper)
	configmapInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			updateConfig(obj.(*v1.ConfigMap))
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			updateConfig(newObj.(*v1.ConfigMap))
		},
		DeleteFunc: func(obj interface{}) {
			config = Config{}
		},
	})
	log.Printf("Starting informer.")
	go configmapInformer.Run(stopper)

	err = setCA(caCert, caKey)
	if err != nil {
		log.Fatalf("Could not set CA: %s", err)
	}

	proxy = goproxy.NewProxyHttpServer()
	proxy.OnRequest(blockedHosts()).HandleConnect(goproxy.AlwaysMitm)
	proxy.OnRequest(blockedHosts()).DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		ctx.Logf("Request: %s %s", req.Method, req.URL)

		if blockedUrls(req, ctx) {
			ctx.Warnf("URL blocked: %s", req.URL)
			return req, goproxy.NewResponse(req,
				goproxy.ContentTypeText, http.StatusForbidden,
				"URL is blocked.")
		}

		return req, nil
	})

	proxy.Verbose = verbose
	log.Fatal(http.ListenAndServe(listenAddr, proxy))
}

func informerListOpts(opts *meta_v1.ListOptions) {
	opts.FieldSelector = fields.OneTermEqualSelector(api.ObjectNameField, configMapName).String()
}

func updateConfig(cm *v1.ConfigMap) {
	log.Printf("Configmap has changed, updating config.")
	err := yaml.Unmarshal([]byte(cm.Data["config"]), &config)
	if err != nil {
		log.Printf("Could not parse config YAML: %s", err)
	}
}

func blockedHosts() goproxy.ReqConditionFunc {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
		host := req.URL.Host

		for _, confURL := range config.BlockedUrls {
			parsedURL, err := url.Parse(confURL.URL)
			if err != nil {
				ctx.Warnf("Could not parse URL %s: %s", confURL.URL, err)
				return false
			}
			if host == parsedURL.Host {
				return true
			}
		}

		return false
	}
}

func blockedUrls(req *http.Request, ctx *goproxy.ProxyCtx) bool {
	reqURL := req.URL.String()
	reqMethod := req.Method

	for _, confURL := range config.BlockedUrls {
		match, _ := regexp.MatchString(confURL.URL, reqURL)
		if reqMethod == confURL.Method && match {
			return true
		}
	}

	return false
}

func kubeConfig(kubeconfig, context string) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}

	if len(context) > 0 {
		overrides.CurrentContext = context
	}

	if len(kubeconfig) > 0 {
		rules.ExplicitPath = kubeconfig
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
}
