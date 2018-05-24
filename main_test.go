package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/emicklei/go-restful"
	"k8s.io/helm/pkg/proto/hapi/services"
)

func deleteRelease(serverUrl, namespace, releaseName string) error {
	var payload = &DeleteReleaseRequest{
		Purge: true,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", serverUrl+"/api/v1/releases/"+releaseName, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", restful.MIME_JSON)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response: %v, expected: %v", resp.StatusCode, http.StatusOK)
	}

	return nil
}

func listReleases(serverUrl, namespace string) ([]string, error) {
	releaseNames := []string{}

	req, err := http.NewRequest("GET", serverUrl+"/api/v1/releases", nil)
	if err != nil {
		return releaseNames, err
	}
	req.Header.Set("Content-Type", restful.MIME_JSON)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return releaseNames, err
	}

	defer resp.Body.Close()

	var listReleasesResponse services.ListReleasesResponse
	err = json.NewDecoder(resp.Body).Decode(&listReleasesResponse)
	if err != nil {
		return releaseNames, err
	}

	for _, r := range listReleasesResponse.Releases {
		releaseNames = append(releaseNames, r.Name)
	}

	return releaseNames, nil
}

func installPackage(serverUrl, namespace string) (string, error) {
	var payload = &InstallReleaseRequest{
		RepoName:  "stable",
		ChartName: "wordpress",
		Version:   "0.8.7",
		Namespace: namespace,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", serverUrl+"/api/v1/releases", bytes.NewBuffer(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", restful.MIME_JSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected response: %v, expected: %v", resp.StatusCode, http.StatusCreated)
	}

	var installReleaseResponse services.InstallReleaseResponse
	err = json.NewDecoder(resp.Body).Decode(&installReleaseResponse)
	if err != nil {
		return "", err
	}

	return installReleaseResponse.Release.Name, nil
}

func TestServer(t *testing.T) {
	serverUrl := "http://localhost:8090"
	go func() {
		RunServer()
	}()
	if err := waitForServerUp(serverUrl); err != nil {
		t.Errorf("%+v\n", err)
	}

	var namespace = "skeg-tests"

	if os.Getenv("TESTS_NAMESPACE") != "" {
		namespace = os.Getenv("TESTS_NAMESPACE")
	}

	releaseName, err := installPackage(serverUrl, namespace)
	if err != nil {
		t.Error(err)
	}

	releases, err := listReleases(serverUrl, namespace)
	if err != nil {
		t.Error(err)
	}
	if len(releases) != 1 {
		t.Error(fmt.Errorf("unexpected number of releases: %d, expected: %d", len(releases), 1))
	}

	err = deleteRelease(serverUrl, namespace, releaseName)
	if err != nil {
		t.Error(err)
	}

	releases, err = listReleases(serverUrl, namespace)
	if err != nil {
		t.Error(err)
	}
	if len(releases) != 0 {
		t.Error(fmt.Errorf("unexpected number of releases: %d, expected: %d", len(releases), 0))
	}
}

func waitForServerUp(serverUrl string) error {
	for start := time.Now(); time.Since(start) < time.Minute; time.Sleep(5 * time.Second) {
		_, err := http.Get(serverUrl + "/")
		if err == nil {
			return nil
		}
	}

	return errors.New("waiting for server timed out")
}

func RunServer() {
	r := NewReleaseResource()
	restful.DefaultContainer.Add(r.WebService())

	log.Fatal(http.ListenAndServe(":8090", nil))
}
