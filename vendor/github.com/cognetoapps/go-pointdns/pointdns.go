package pointdns

import (
    "fmt"
    "net/http"
    "io"
    "io/ioutil"
    "encoding/json"
    "strings"
    "errors"
)

const (
    ApiUrl = "https://pointhq.com"
    ApiVersion = ""
)

type PointClient struct {
    Email string
    ApiToken string
    HttpClient *http.Client
}

type Resource interface {
    Id() int
}

func NewClient(email, apiToken string) *PointClient {
    return &PointClient{Email: email, ApiToken: apiToken, HttpClient: &http.Client{}}
}

func (client *PointClient) DoRequest (method, path string, body io.Reader) (string, int, error) {
    url := ApiUrl + fmt.Sprintf("%s/%s", ApiVersion, path)
    req, _ := http.NewRequest(method, url, body)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Add("Accept", "application/json")
    req.SetBasicAuth(client.Email, client.ApiToken)
    resp, err := client.HttpClient.Do(req)
    if err != nil {
        return "", 0, err
    }

    defer resp.Body.Close()
    responseBytes, _ := ioutil.ReadAll(resp.Body)
    return string(responseBytes), resp.StatusCode, nil
}

func (client *PointClient) Get (path string, val interface{}) error {
    body, status, err := client.DoRequest("GET", path, nil)
    if err != nil {
        return err
    }
    if status != http.StatusOK {
        return errors.New(fmt.Sprintf("Error occurred: %s", body))
    }
    json.Unmarshal([]byte(body), &val)
    return nil
}

func (client *PointClient) Delete(path string, val interface{}) error {
    body, status, err := client.DoRequest("DELETE", path, nil)
    if err != nil {
        return err
    }
    if status != http.StatusAccepted {
        return errors.New(fmt.Sprintf("Error occurred: %s", body))
    }
    return nil
}

func persisted (id int) bool {
    if id > 0 {
        return true
    }
    return false
}

func (client *PointClient) Save (path string, payload Resource, val interface{}) error {
    method := "POST"
    if persisted(payload.Id()) {
        method = "PUT"
    }
    jsonPayload, err := json.Marshal(payload)

    body, status, err := client.DoRequest(method, path, strings.NewReader(string(jsonPayload)))
    if err != nil {
        return err
    }
    if status != http.StatusAccepted && status != http.StatusCreated {
        return errors.New(fmt.Sprintf("Error occurred: %s", body))
    }
    json.Unmarshal([]byte(body), &val)
    return nil
}

