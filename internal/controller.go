package health

import (
	"fmt"
	"net/http"
)

type Controller struct{}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) HealthHandler(w http.ResponseWriter, r *http.Request) {
	_, err := fmt.Fprint(w, "ok")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
