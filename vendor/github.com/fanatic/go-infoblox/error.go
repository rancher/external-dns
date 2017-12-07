package infoblox

import (
  "fmt"
)

type Error map[string]interface{}

func (e Error) Message() string {
  return e["Error"].(string)
}

func (e Error) Code() string {
  return e["code"].(string)
}

func (e Error) Text() string {
  return e["text"].(string)
}

func (e Error) Error() string {
  return fmt.Sprintf("Error %s - %s - %s", e.Message(), e.Code(), e.Text())
}

type Errors map[string]interface{}

func (e Errors) Error() string {
  var (
    msg string = ""
    err Error
    ok  bool
  )
  for _, val := range e["errors"].([]interface{}) {
    if err, ok = val.(map[string]interface{}); ok {
      msg += err.Error() + ". "
    }
  }
  return msg
}

func (e Errors) String() string {
  return e.Error()
}

func (e Errors) Errors() []Error {
  var errs = e["errors"].([]interface{})
  var out = make([]Error, len(errs))
  for i, val := range errs {
    out[i] = Error(val.(map[string]interface{}))
  }
  return out
}
