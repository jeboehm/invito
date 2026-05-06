package handler

import "net/http"

type WidgetHandler struct {
	pub *PublicHandler
}

func NewWidgetHandler(pub *PublicHandler) *WidgetHandler {
	return &WidgetHandler{pub: pub}
}

func (h *WidgetHandler) HandleWidgetSlotPicker(w http.ResponseWriter, r *http.Request) {
	h.pub.handleSlotPicker(w, r, "booking/widget.html", "booking/widget-slots-partial.html")
}

func (h *WidgetHandler) HandleWidgetBookingSubmit(w http.ResponseWriter, r *http.Request) {
	h.pub.handleBookingSubmit(w, r, "booking/widget-confirm.html")
}
