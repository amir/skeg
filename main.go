package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/emicklei/go-restful"
	"github.com/spf13/pflag"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
)

type ReleaseResource struct {
	client   *helm.Client
	settings helm_env.EnvSettings
}

type InstallReleaseRequest struct {
	RepoName  string `json:"repoName" description:"repository name"`
	ChartName string `json:"chartName" description:"chart name"`
	Version   string `json:"version" description:"chart version"`
	Namespace string `json:"namespace" description:"namespace"`
}

type DeleteReleaseRequest struct {
	DryRun       bool  `json:"dryRun" description:"simulate a delete"`
	DisableHooks bool  `json:"disableHooks" description:"prevent hooks from running during deletion"`
	Purge        bool  `json:"purge" description:"remove the release from the store and make its name free for later user"`
	Timeout      int64 `json:"timeout" description:"time in seconds to wait for any individual Kubernetes operations"`
}

var (
	tillerAddr  string
	serviceAddr string
)

func (r *ReleaseResource) Install(request *restful.Request, response *restful.Response) {
	req := new(InstallReleaseRequest)
	err := request.ReadEntity(&req)
	if err == nil {
		installOptions := []helm.InstallOption{
			helm.ValueOverrides([]byte("")),
		}
		dl := downloader.ChartDownloader{
			HelmHome: r.settings.Home,
			Getters:  getter.All(r.settings),
		}
		filename, _, err := dl.DownloadTo(
			fmt.Sprintf("%s/%s", req.RepoName, req.ChartName),
			req.Version,
			r.settings.Home.Archive(),
		)
		if err != nil {
			response.WriteError(http.StatusInternalServerError, err)
		} else {
			resp, err := r.client.InstallRelease(filename, req.Namespace, installOptions...)
			if err != nil {
				response.WriteError(http.StatusInternalServerError, err)
			} else {
				response.WriteHeaderAndEntity(http.StatusCreated, resp)
			}
		}
	} else {
		response.WriteError(http.StatusBadRequest, err)
	}
}

func (r *ReleaseResource) List(request *restful.Request, response *restful.Response) {
	resp, err := r.client.ListReleases()
	if err == nil {
		response.WriteEntity(resp)
	} else {
		response.WriteError(http.StatusInternalServerError, err)
	}
}

func (r *ReleaseResource) Delete(request *restful.Request, response *restful.Response) {
	req := new(DeleteReleaseRequest)
	err := request.ReadEntity(&req)
	if err == nil {
		releaseName := request.PathParameter("release-name")
		deleteOptions := []helm.DeleteOption{
			helm.DeleteDryRun(req.DryRun),
			helm.DeleteDisableHooks(req.DisableHooks),
			helm.DeletePurge(req.Purge),
			helm.DeleteTimeout(req.Timeout),
		}
		resp, err := r.client.DeleteRelease(releaseName, deleteOptions...)
		if err != nil {
			response.WriteError(http.StatusInternalServerError, err)
		} else {
			response.WriteEntity(resp)
		}
	} else {
		response.WriteError(http.StatusBadRequest, err)
	}
}

func (r *ReleaseResource) WebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/api/v1/releases").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	ws.
		Route(ws.POST("/").To(r.Install).
			Doc("Install a release").
			Reads(InstallReleaseRequest{}))

	ws.
		Route(ws.GET("/").To(r.List).
			Doc("List releases"))

	ws.
		Route(ws.DELETE("/{release-name}").To(r.Delete).
			Doc("Delete a releases").
			Param(ws.PathParameter("releases-name", "release name").DataType("string")))

	return ws
}

func NewReleaseResource() *ReleaseResource {
	options := []helm.Option{
		helm.Host(tillerAddr),
		helm.ConnectTimeout(10),
	}
	client := helm.NewClient(options...)

	var settings helm_env.EnvSettings
	flagSet := pflag.NewFlagSet("", pflag.PanicOnError)
	settings.AddFlags(flagSet)

	return &ReleaseResource{client: client, settings: settings}
}

func init() {
	flag.StringVar(&tillerAddr, "tillerAddr", "localhost:44134", "Tiller host:port")
	flag.StringVar(&serviceAddr, "addr", ":8080", "Service address")
}

func main() {
	r := NewReleaseResource()
	restful.DefaultContainer.Add(r.WebService())

	log.Fatal(http.ListenAndServe(serviceAddr, nil))
}
