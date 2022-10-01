package engine

import (
	"net/http"
	"strconv"
)

func NewHealthRouter() *HealthRouter {
	return &HealthRouter{
		checked: 0,
	}
}

type HealthRouter struct {
	checked int64
}

func (r *HealthRouter) StartWebServer(address string) {
	r.registerRoutes()
	go func() {
		err := http.ListenAndServe(address, nil)
		if err != nil {
			L().Error(err)
		}
	}()
}

func (r *HealthRouter) registerRoutes() {
	http.HandleFunc("/health", r.onHealthCheckRequest)
	http.HandleFunc("/healthcount", r.onHealthCheckCountRequest)
}

func (r *HealthRouter) onHealthCheckRequest(writer http.ResponseWriter, req *http.Request) {
	r.checked += 1
	_, err := writer.Write([]byte("ok"))
	if err != nil {
		L().Error(err)
	}
}

func (r *HealthRouter) onHealthCheckCountRequest(writer http.ResponseWriter, req *http.Request) {
	_, err := writer.Write([]byte(strconv.FormatInt(r.checked, 10)))
	if err != nil {
		L().Error(err)
	}
}
