package server

import (
	"net/http"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const libreChatRetiredCode = "librechat_retired"

func newLibreChatRetiredHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assistantUILog(r, assistantRuntimeReasonRetiredByDesign)
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusGone, libreChatRetiredCode, "librechat_retired")
	})
}
