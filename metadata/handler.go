package metadata

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type MetadataHandler struct {
	url string
}

func NewHandler(url string) MetadataHandler {
	return MetadataHandler{url}
}

func (m *MetadataHandler) SendRequest(path string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", m.url+"/latest"+path, nil)
	req.Header.Add("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (m *MetadataHandler) GetVersion() (string, error) {
	resp, err := m.SendRequest("/version")
	if err != nil {
		return "", err
	}
	return string(resp[:]), nil
}

func (m *MetadataHandler) GetStack() (Stack, error) {
	resp, err := m.SendRequest("/self/stack")
	var stack Stack
	if err != nil {
		return stack, err
	}
	err = json.Unmarshal(resp, &stack)
	if err != nil {
		return stack, err
	}

	return stack, nil
}

func (m *MetadataHandler) GetServices() ([]Service, error) {
	resp, err := m.SendRequest("/services")
	var services []Service
	if err != nil {
		return services, err
	}

	err = json.Unmarshal(resp, &services)
	if err != nil {
		return services, err
	}
	return services, nil
}

func (m *MetadataHandler) GetContainers() ([]Container, error) {
	resp, err := m.SendRequest("/containers")
	var containers []Container
	if err != nil {
		return containers, err
	}

	err = json.Unmarshal(resp, &containers)
	if err != nil {
		return containers, err
	}
	return containers, nil
}
