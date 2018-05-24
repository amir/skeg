package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/emicklei/go-restful"
	"github.com/spf13/pflag"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
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

type UpdateReleaseRequest struct {
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

func (r *ReleaseResource) Update(request *restful.Request, response *restful.Response) {
	req := new(UpdateReleaseRequest)
	err := request.ReadEntity(&req)
	if err == nil {
		releaseName := request.PathParameter("release-name")
		updateOptions := []helm.UpdateOption{
			helm.UpdateValueOverrides([]byte("")),
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
			resp, err := r.client.UpdateRelease(releaseName, filename, updateOptions...)
			if err != nil {
				response.WriteError(http.StatusInternalServerError, err)
			} else {
				response.WriteHeaderAndEntity(http.StatusOK, resp)
			}
		}

	} else {
		response.WriteError(http.StatusBadRequest, err)
	}
}

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

func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}

	return false
}

func releaseStatusCodes(code string) []release.Status_Code {
	if code == "all" {
		return []release.Status_Code{
			release.Status_UNKNOWN,
			release.Status_DEPLOYED,
			release.Status_DELETED,
			release.Status_DELETING,
			release.Status_FAILED,
			release.Status_PENDING_INSTALL,
			release.Status_PENDING_UPGRADE,
			release.Status_PENDING_ROLLBACK,
		}
	}

	statusCodes := []release.Status_Code{}
	codes := strings.Split(code, ",")

	if contains(codes, "deployed") {
		statusCodes = append(statusCodes, release.Status_DEPLOYED)
	}

	if contains(codes, "deleted") {
		statusCodes = append(statusCodes, release.Status_DELETED)
	}

	if contains(codes, "deleting") {
		statusCodes = append(statusCodes, release.Status_DELETING)
	}

	if contains(codes, "failed") {
		statusCodes = append(statusCodes, release.Status_FAILED)
	}

	if contains(codes, "superseded") {
		statusCodes = append(statusCodes, release.Status_SUPERSEDED)
	}

	if contains(codes, "pending") {
		statusCodes = append(statusCodes, release.Status_PENDING_INSTALL, release.Status_PENDING_UPGRADE, release.Status_PENDING_ROLLBACK)
	}

	return statusCodes
}

func listOptions(request *restful.Request) []helm.ReleaseListOption {
	sortBy := services.ListSort_NAME
	if request.QueryParameter("sort_by") == "last_released" {
		sortBy = services.ListSort_LAST_RELEASED
	}

	sortOrder := services.ListSort_ASC
	if request.QueryParameter("sort_ord") == "desc" {
		sortOrder = services.ListSort_DESC
	}

	limit := 256
	if l, err := strconv.Atoi(request.QueryParameter("limit")); err == nil {
		limit = l
	}

	statusCodes := []release.Status_Code{
		release.Status_DEPLOYED,
		release.Status_FAILED,
	}

	if request.QueryParameter("status") != "" {
		statusCodes = releaseStatusCodes(request.QueryParameter("status"))
	}

	return []helm.ReleaseListOption{
		helm.ReleaseListSort(int32(sortBy)),
		helm.ReleaseListOrder(int32(sortOrder)),
		helm.ReleaseListLimit(limit),
		helm.ReleaseListOffset(request.QueryParameter("offset")),
		helm.ReleaseListFilter(request.QueryParameter("filter")),
		helm.ReleaseListStatuses(statusCodes),
		helm.ReleaseListNamespace(request.QueryParameter("namespace")),
	}
}

func (r *ReleaseResource) List(request *restful.Request, response *restful.Response) {
	resp, err := r.client.ListReleases(listOptions(request)...)
	if err == nil {
		response.WriteEntity(resp)
	} else {
		if err != io.EOF {
			response.WriteError(http.StatusInternalServerError, err)
		} else {
			response.WriteEntity(&services.ListReleasesResponse{})
		}
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
			Doc("List releases").
			Param(ws.QueryParameter("sort_by", "sort by").DataType("string")).
			Param(ws.QueryParameter("sort_ord", "sort order").DataType("string")).
			Param(ws.QueryParameter("limit", "limit").DataType("int")).
			Param(ws.QueryParameter("offset", "offset").DataType("int")).
			Param(ws.QueryParameter("filter", "filter").DataType("string")).
			Param(ws.QueryParameter("status", "status").DataType("string")))

	ws.
		Route(ws.DELETE("/{release-name}").To(r.Delete).
			Doc("Delete a releases").
			Param(ws.PathParameter("releases-name", "release name").DataType("string")))

	ws.
		Route(ws.POST("/{release-name}").To(r.Update).
			Doc("Update a releases").
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
