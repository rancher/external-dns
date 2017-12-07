package infoblox

import (
  "compress/gzip"
  "encoding/json"
  "fmt"
  "io"
  "io/ioutil"
  "net/http"
  "strings"
)

const (
  STATUS_OK           = 200
  STATUS_CREATED      = 201
  STATUS_INVALID      = 400
  STATUS_UNAUTHORIZED = 401
  STATUS_FORBIDDEN    = 403
  STATUS_NOTFOUND     = 404
  STATUS_LIMIT        = 429
  STATUS_GATEWAY      = 502
)

// APIResponse is used to parse the response from Infoblox.
// GET requests tend to respond with Objects or lists of Objects
// while POST,PUT,DELETE returns the Object References as a string
// https://192.168.2.200/wapidoc/#get
type APIResponse http.Response

func (r APIResponse) readBody() (b []byte, err error) {
  var (
    header string
    reader io.Reader
  )
  header = strings.ToLower(r.Header.Get("Content-Encoding"))
  if header == "" || strings.Index(header, "gzip") == -1 {
    reader = r.Body
    defer r.Body.Close()
  } else {
    if reader, err = gzip.NewReader(r.Body); err != nil {
      return
    }
  }
  b, err = ioutil.ReadAll(reader)
  return
}

func (r APIResponse) ReadBody() string {
  var (
    b   []byte
    err error
  )
  if b, err = r.readBody(); err != nil {
    return ""
  }
  return string(b)
}

// Parses a JSON encoded HTTP response into the supplied interface.
func (r APIResponse) Parse(out interface{}) (err error) {
  var b []byte
  switch r.StatusCode {
  case STATUS_UNAUTHORIZED:
    fallthrough
  case STATUS_NOTFOUND:
    fallthrough
  case STATUS_GATEWAY:
    fallthrough
  case STATUS_FORBIDDEN:
    fallthrough
  case STATUS_INVALID:
    e := &Error{}
    if b, err = r.readBody(); err != nil {
      return
    }
    if err = json.Unmarshal(b, e); err != nil {
      err = fmt.Errorf("Error parsing error response: %v", string(b))
    } else {
      err = *e
    }
    return
  //case STATUS_LIMIT:
  //  err = RateLimitError{
  //    Limit:     r.RateLimit(),
  //    Remaining: r.RateLimitRemaining(),
  //    Reset:     r.RateLimitReset(),
  //  }
  //  return
  case STATUS_CREATED:
    fallthrough
  case STATUS_OK:
    if b, err = r.readBody(); err != nil {
      return
    }
    err = json.Unmarshal(b, out)
    if err == io.EOF {
      err = nil
    }
  default:
    if b, err = r.readBody(); err != nil {
      return
    }
    err = fmt.Errorf("%v", string(b))
  }
  return
}
