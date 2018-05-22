package main

import (
	"sync"
	"testing"

	"github.com/spf13/pflag"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	helm_env "k8s.io/helm/pkg/helm/environment"
)

const N = 20

func TestConcurrentDownloads(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(N)

	var settings helm_env.EnvSettings
	flagSet := pflag.NewFlagSet("", pflag.PanicOnError)
	settings.AddFlags(flagSet)

	dl := downloader.ChartDownloader{
		HelmHome: settings.Home,
		Getters:  getter.All(settings),
	}

	for i := 1; i <= N; i++ {
		go func() {
			defer wg.Done()
			f, _, err := dl.DownloadTo("stable/wordpress", "0.8.7", settings.Home.Archive())

			if err != nil {
				t.Error(err)
			}

			_, err = chartutil.Load(f)
			if err != nil {
				t.Error(err)
			}
		}()
	}

	wg.Wait()
}
